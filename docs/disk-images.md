# Disk images

This page describes the classic Mac disk formats that scriptview reads, as far as it needs them: where the volume is, and how to find each file's forks. Apple documented HFS in Inside Macintosh: Files and HFS+ in Technical Note TN1150. scriptview was tested on two hard disk images and on synthetic volumes. A 262 MB System 7.1 disk has 3,711 files on HFS, with 40 scripts and stacks. A 10 GB Mac OS 9.1 disk has 6,028 files on HFS+, with 227 scripts and stacks. scriptview read every file's forks without an error, and finds the scripts on the larger disk in 15 seconds. On the Mac OS 8.6 Japanese install CD, it finds 131 scripts and stacks in under a second, 52 of them in a Disk Copy 6 image on the CD, with their Japanese names. A copy of a Logitech driver CD that ends before its HFS partition does gets an error that says so.

All numbers are big-endian.

## Containers

scriptview looks for a volume in this order:

- A bare volume: a classic HFS volume has `BD` at offset 1024, an HFS+ volume `H+` (or `HX`, for the case-sensitive variant).
- An Apple partition map, on hard disks and CDs: the driver descriptor at offset 0 has `ER` and the block size at 2. From block 1, each block holds one 512-byte map entry, with `PM` at 0, the number of entries at 4, the partition's start and length in blocks at 8 and 12, and its type at 48, such as `Apple_HFS`. A CD that's also an ISO 9660 disc for PCs has both. scriptview opens every HFS partition.
- A Disk Copy 4.2 image: the disk's name as a Pascal string in the first 64 bytes, the disk's size at 64, `01 00` at 82, and the disk from 84.
- A MacBinary file: a 128-byte header with the name's length at 1, and the data fork's size at 83, then the data fork, which holds one of the above.

- A GUID partition table, on Intel Macs' disks and in `.dmg` images: `EFI PART` at 512, with the entries' first sector at 72, their number at 80, and their size at 84, all little-endian. Each entry has its first and last sectors at 32 and 40. scriptview opens any partition that holds an HFS or HFS+ volume. APFS volumes aren't read.

## Compressed images

Two formats store a disk in chunks, each a run of 512-byte sectors that's stored as is, compressed, or left out because it's all zeros. scriptview decompresses one chunk at a time as it reads, so a large image isn't expanded in memory.

A Disk Copy 6 image (NDIF, usually `.img`, or `.smi` when it mounts itself) keeps its layout in its resource fork, in resource `bcem` 128, so scriptview needs the resource fork too: on a Mac, in the `._name` file, in a MacBinary file, or on a disk image. The resource holds the disk's size in sectors at 0x44, the number of chunks at 0x7C, and from 0x80, 12 bytes for each chunk: its first sector, 3 bytes, its kind, then its data's offset and length in the data fork, 4 bytes each. Kind 2 is stored as is, 0x83 is compressed with Apple Data Compression (ADC), 0 and 1 are zeros, and 0xFF ends the list. A chunk runs to the next one's first sector. Check the layout before the data fork: a data fork that starts with an uncompressed chunk has the disk's first sectors, which look like a bare volume.

A `.dmg` image (UDIF), from Mac OS X on, ends with a 512-byte trailer, `koly`, with the data fork's offset at 24, and the offset and length of an XML property list at 216 and 224. The list's `blkx` entries are base64 data, one chunk table, `mish`, for each partition. A table has its first sector at 8, an offset for its chunks' data at 24, its number of chunks at 200, and from 204, 40 bytes for each chunk: its kind, its first sector and count at 8 and 16, counted from the table's first sector, and its data's offset and length at 24 and 32, counted from the data fork's offset plus the table's. The kinds:

- 1: stored as is.
- 0 and 2: zeros.
- 0x80000004: ADC.
- 0x80000005: zlib.
- 0x80000006: bzip2.
- 0x80000007 and 0x80000008: LZFSE and LZMA, from later versions of macOS, which scriptview can't read yet.
- 0x7FFFFFFE and 0xFFFFFFFF: a comment and the end, which hold no data.

ADC is a simple LZ77 scheme. Each run starts with a byte. If its high bit is set, the low 7 bits plus 1 count literal bytes that follow. If the next bit is set, the low 6 bits plus 4 count bytes to copy from earlier output, and the next 2 bytes plus 1 say how far back. Otherwise, bits 2 to 5 plus 3 count bytes to copy, and the low 2 bits with the next byte, plus 1, say how far back.

## HFS

The master directory block at offset 1024 gives:

- At 0x14: the allocation block size, 4 bytes.
- At 0x1C: where allocation block 0 starts, in 512-byte sectors, 2 bytes.
- At 0x24: the volume's name, a Pascal string of up to 27 characters.
- At 0x7C: `H+` when the volume wraps an HFS+ volume, whose start and length in allocation blocks follow at 0x7E and 0x80. Mac OS 8.1 to 9 formatted disks this way, so that older Macs could start from them.
- At 0x82 and 0x92: the sizes of the extents overflow file and the catalog file, 4 bytes each, each followed by its first three extents, a start block and a block count of 2 bytes each.

The catalog is a B-tree, keyed by a folder's ID and a name. Its leaf records describe folders and files:

- A folder record has type 1 at 0, and the folder's ID at 6. The root folder's ID is 2.
- A file record has type 2 at 0, the type and creator at 4 and 8, its ID at 20, the data fork's size at 26 and the resource fork's at 36, and the first three extents of each fork at 74 and 86.

A fork with more than three extents keeps the rest in the extents overflow file, another B-tree, keyed by the fork (0 for data, 0xFF for resources), the file's ID, and the fork's first block in the record.

Names are bytes in the Mac encoding of the system that wrote the volume, so a Japanese disk has Mac Japanese names. scriptview decodes them in the encoding that their text suggests. See [Text encodings](text-encodings.md).

## HFS+

The volume header at offset 1024 gives the block size at 40, 4 bytes, and the forks of the extents overflow file at 192 and the catalog file at 272. A fork's record is its size, 8 bytes, then from 16, eight extents of a 4-byte start block and count. Allocation block 0 starts at the volume's start.

The catalog's keys hold names in UTF-16. A folder record has type 1 at 0 and its ID at 8. A file record has type 2, its ID at 8, the type and creator at 48 and 52, and its data and resource forks at 88 and 168. Files in folders whose names start with NUL bytes are hard links' targets, which the Finder hides, and scriptview skips them.

## B-trees

The catalog and the extents overflow file share a B-tree layout in both formats. Each node starts with a 14-byte descriptor, with the next node in its level at 0, the kind at 8 (-1 for a leaf, 1 for the header), and the number of records at 10. A node's last bytes are 2-byte offsets of its records, from the end backward, with one more for the start of the free space. Node 0 is the header node, whose record gives the first leaf node at 24 and the node size at 32, counting from the node's start. scriptview reads every record by following the leaf nodes from the first.
