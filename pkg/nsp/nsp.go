package nsp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	// Magic number that identifies the PFS0 file format
	PFS0Magic = "PFS0"
	// Default buffer size (in bytes) for file I/O operations
	DefaultBufferSize = 4096
)

// Builder provides functionality for creating Nintendo Submission Package (NSP)
// files. The Builder collects input files and handles the creation of a
// properly formatted NSP.
type Builder struct {
	// Path where the NSP file will be created
	OutputPath string

	// Controls the buffer size used for file I/O operations.
	// Larger values may improve performance at the cost of memory usage.
	BufferSize int

	// Determines whether to display progress indicators
	ShowProgress bool

	// Progress update frequency in milliseconds
	ProgressUpdateFrequency int

	// Track last output length for clean line clearing
	lastProgressWidth int

	// The collection of files to be included in the NSP
	partEntries []partitionEntry
}

// partitionEntry contains metadata about a file to be included in the NSP
type partitionEntry struct {
	// Filesystem path to the source file
	path string

	// Filename to be stored in the NSP
	name string

	// Size of the file in bytes
	size uint64

	// Absolute position of file data in the NSP
	dataOffset uint64

	// Offset of this file's name in the string table
	stringOffset uint32
}

// AddFile adds a file to be included in the NSP
func (b *Builder) AddFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to add file %s: %w", path, err)
	}

	b.partEntries = append(b.partEntries, partitionEntry{
		path: path,
		name: info.Name(),
		size: uint64(info.Size()),
	})

	return nil
}

// AddFiles adds multiple files to be included in the NSP.
func (b *Builder) AddFiles(paths []string) error {
	for _, path := range paths {
		if err := b.AddFile(path); err != nil {
			return err
		}
	}
	return nil
}

// Build creates the NSP file with all the added files.
func (b *Builder) Build() error {
	if len(b.partEntries) == 0 {
		return fmt.Errorf("no input files provided")
	}

	sort.Slice(b.partEntries, func(i, j int) bool {
		return b.partEntries[i].name < b.partEntries[j].name
	})

	header := b.generateHeader()

	outFile, err := os.Create(b.OutputPath)
	if err != nil {
		return fmt.Errorf(
			"failed to create output file %s: %w",
			b.OutputPath,
			err,
		)
	}
	defer outFile.Close()

	bytesWritten, err := outFile.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	} else if bytesWritten != len(header) {
		return fmt.Errorf(
			"size mismatch for file %s during write: "+
				"expected %d bytes, wrote %d bytes",
			b.OutputPath,
			len(header),
			bytesWritten,
		)
	}

	var totalSize uint64
	for _, file := range b.partEntries {
		totalSize += file.size
	}

	// Initialize progress tracking
	var processedSize uint64
	buffer := make([]byte, b.BufferSize)

	if b.ShowProgress {
		fmt.Printf("Building NSP: %s\n", b.OutputPath)
	}

	for i, file := range b.partEntries {
		if b.ShowProgress {
			fmt.Printf(
				"Processing (%d/%d): %s\n",
				i+1,
				len(b.partEntries),
				file.name,
			)
		}

		err := b.copyFileToNSP(outFile, file, buffer, &processedSize, totalSize)
		if b.ShowProgress {
			fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// generateHeader creates the PFS0 header for the NSP file.
// The header consists of:
// 0x0-0x4:   Magic ("PFS0")
// 0x4-0x8:   EntryCount (number of files)
// 0x8-0xC:   StringTableSize (including padding)
// 0xC-0x10:  Reserved (zeros)
// 0x10-X:    PartitionEntryTable (24 bytes per file)
// X-Y:       StringTable with null-terminated filenames
func (b *Builder) generateHeader() []byte {
	stringTableSize := 0
	for _, file := range b.partEntries {
		stringTableSize += len(file.name) + 1 // +1 for null terminator
	}

	// Magic(4) + EntryCount(4) + StringTableSize(4) + Reserved(4)
	headerMetadataSize := 0x10

	entryCount := len(b.partEntries)
	partitionTableSize := entryCount * 0x18 // 24 bytes per entry
	headerSize := headerMetadataSize + partitionTableSize + stringTableSize

	// Align to 16 bytes if needed
	padding := 0
	if remainder := headerSize % 0x10; remainder > 0 {
		padding = 0x10 - remainder
		headerSize += padding
	}

	header := make([]byte, headerSize)

	// Write magic "PFS0" at 0x0
	copy(header[0:], PFS0Magic)

	// Write EntryCount at 0x4
	binary.LittleEndian.PutUint32(header[4:], uint32(entryCount))

	// Write StringTableSize at 0x8 (including padding)
	binary.LittleEndian.PutUint32(
		header[8:],
		uint32(stringTableSize+padding),
	)

	// Reserved field at 0xC (4 bytes of zeros)
	binary.LittleEndian.PutUint32(header[12:], 0)

	// Write PartitionEntryTable starting at 0x10
	headerPosition := 0x10
	stringOffset := uint32(0)
	fileDataOffset := uint64(0)

	// Calculate the base offset for file data (after header)
	fileDataBaseOffset := uint64(headerSize)
	for i := range b.partEntries {
		file := &b.partEntries[i]

		// File data offset (relative to start of file data section)
		binary.LittleEndian.PutUint64(
			header[headerPosition:],
			fileDataOffset,
		)
		// Store absolute position for later use
		file.dataOffset = fileDataBaseOffset + fileDataOffset
		fileDataOffset += file.size
		headerPosition += 8

		// File size
		binary.LittleEndian.PutUint64(
			header[headerPosition:],
			file.size,
		)
		headerPosition += 8

		// StringOffset (position in string table)
		binary.LittleEndian.PutUint32(
			header[headerPosition:],
			stringOffset,
		)
		file.stringOffset = stringOffset
		stringOffset += uint32(
			len(file.name) + 1, // +1 for null terminator
		)
		headerPosition += 4

		// Reserved field (4 bytes of zeros)
		binary.LittleEndian.PutUint32(header[headerPosition:], 0)
		headerPosition += 4
	}

	// Write string table (filenames with null terminators)
	stringTableOffset := headerMetadataSize + partitionTableSize
	for _, entry := range b.partEntries {
		nameOffset := stringTableOffset + int(entry.stringOffset)
		copy(header[nameOffset:], entry.name)
		// The buffer is already filled with zeros, so null terminators are
		// implicit
	}

	return header
}

// copyFileToNSP copies a file to the NSP output at the position indicated by
// its dataOffset.
func (b *Builder) copyFileToNSP(
	outFile *os.File,
	fileInfo partitionEntry,
	buffer []byte,
	processedSize *uint64,
	totalSize uint64,
) error {
	inFile, err := os.Open(fileInfo.path)
	if err != nil {
		return fmt.Errorf(
			"failed to open input file %s: %w",
			fileInfo.path,
			err,
		)
	}
	defer inFile.Close()

	// Seek to the correct position in the output file
	if _, err := outFile.Seek(int64(fileInfo.dataOffset), io.SeekStart); err != nil {
		return fmt.Errorf(
			"failed to seek in output file %s: %w",
			fileInfo.path,
			err,
		)
	}

	bytesWritten := uint64(0)
	lastUpdateTime := time.Now()
	total := float64(0)
	totalUnit := ""

	if b.ShowProgress {
		total, totalUnit = formatSize(totalSize)
	}

	// Copy file data in chunks
	for {
		n, err := inFile.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf(
				"error reading input file %s: %w",
				fileInfo.path,
				err,
			)
		}

		if n == 0 {
			break
		}

		if _, err := outFile.Write(buffer[:n]); err != nil {
			return fmt.Errorf(
				"error writing to output file %s: %w",
				b.OutputPath,
				err,
			)
		}

		bytesWritten += uint64(n)
		*processedSize += uint64(n)

		if b.ShowProgress &&
			time.Since(lastUpdateTime).
				Milliseconds() >=
				int64(
					b.ProgressUpdateFrequency,
				) {
			b.drawProgressBar(*processedSize, totalSize, total, totalUnit, 50)
			lastUpdateTime = time.Now()
		}
	}

	if bytesWritten != fileInfo.size {
		return fmt.Errorf(
			"size mismatch for file %s during write: expected %d bytes, wrote %d bytes",
			fileInfo.path,
			fileInfo.size,
			bytesWritten,
		)
	}

	return nil
}

// clearLine clears the current line in the terminal
func (b *Builder) clearLine() {
	fmt.Print("\r" + strings.Repeat(" ", b.lastProgressWidth) + "\r")
	b.lastProgressWidth = 0
}

// drawProgressBar displays a progress bar showing the current copying progress
func (b *Builder) drawProgressBar(
	currentSize uint64,
	totalSize uint64,
	total float64,
	totalUnit string,
	width int,
) {
	if !b.ShowProgress {
		return
	}

	if totalSize == 0 {
		totalSize = 1 // Avoid division by zero
	}

	percent := float64(currentSize) / float64(totalSize)
	filled := min(int(percent*float64(width)), width)

	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)

	current, currentUnit := formatSize(currentSize)

	progressString := fmt.Sprintf("\r[%s] %5.1f%% (%3.2f %s/%3.2f %s)",
		bar,
		percent*100,
		current,
		currentUnit,
		total,
		totalUnit,
	)
	b.clearLine()
	fmt.Print(progressString)
	b.lastProgressWidth = len(progressString)
}

// formatSize converts a byte size to a human-readable format
func formatSize(bytes uint64) (float64, string) {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1000 && unitIndex < len(units)-1 {
		size /= 1000
		unitIndex++
	}

	return size, units[unitIndex]
}
