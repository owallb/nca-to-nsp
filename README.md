# nca-to-nsp

A utility to package NCA files into a NSP using the PFS0 format.

This tool works by:
1. Reading the specified NCA files
2. Creating a PFS0 header
3. Combining the header with the NCA files into a single NSP file

## Installation

### Prerequisites

- Go 1.24.2 or higher

### Building from source

```bash
git clone https://github.com/owallb/nca-to-nsp.git
cd nca-to-nsp
go build -o nca-to-nsp ./cmd/nca-to-nsp
```

## Usage

Basic usage:

```bash
./nca-to-nsp -o <output-file.nsp> file1.nca [file2.nca ...]
```

### Command-line Options

| Flag | Description |
|------|-------------|
| `-o` | NSP output file name (default: out.nsp) |
| `-buffer` | Buffer size for file copying operations (default: 4096) |
| `-progress` | Show progress bar |
| `-h` | Display help information |
| `-v` | Display version information |

### Example

Create an NSP file from NCA files with progress bar enabled:
```bash
./nca-to-nsp -o out.nsp -progress path/to/dir/*.nca
```

## License

BSD-3-Clause, see [LICENSE](LICENSE) for more information.

## Acknowledgments

- [nspBuild](https://github.com/CVFireDragon/nspBuild) for inspiration on how to pack NCAs into NSP.
