# Testing the classic formats

AppleScript has a reference to test against: `osadecompile` prints any compiled script, and scriptview's output must match it. See [Testing](applescript/testing.md). The classic formats have no such reference that the tests can run, so scriptview is tested on large sets of real files in other ways:

- Every file must read without a crash or an error.
- The parts of a file that repeat each other must agree, such as a stack's header and its card list.
- Where a separate method can find the same thing, the two must agree.
- Each change is checked against the previous build's output on every file.

There are also tests with synthetic files, and fuzzing.

## Corpora

The files come from old software CDs, magazine discs, and online archives, mostly through discmaster.textfiles.com, which lists the files on thousands of disc images by format, and archive.org. Each file is named by the first 12 hex digits of its SHA-1 hash, which removes duplicates. The files belong to their authors, so they aren't in the repository. The tests in the repository use synthetic files.

### Stacks

There are 38,879 distinct stacks:

- From discmaster: about 34,000, every distinct stack of 3 MB or less in its HyperCard listing, which has 106,807 entries.
- From HyperCard Online, the `hypercardstacks` collection on archive.org: 2,470 more, from its 3,618 items.
- From Japanese discs, such as JHUG 94 Vol. 3 and JHUG CD-ROM '96: about 1,200.
- From European magazine discs: 54.
- For the Apple IIGS: 724, from discmaster, the Asimov archive's FTP mirror, a European IIGS collection, and the Apple IIGS System Software beta CD.

By format, 28,704 are from HyperCard 2.x, 9,375 from HyperCard 1.x, 724 from HyperCard IIGS, and 76 from other programs, such as Plus. The first rules came from a smaller set: 29 stacks from Apple's discs of 1993 and 1994, then 4,651 from discmaster and HyperCard Online.

### Projects

There are 876 distinct SuperCard projects and standalones, 142 of them with a resource fork. 745 come from 89 discmaster discs, and 131 from the Wayback Machine. The projects were found by SuperCard's file types. The standalones, 429 of them, are applications with any creator, so they were found by fetching the first 32 bytes of every application's data fork on those discs and looking for a project header. 189 of them are PowerPC or fat standalones. 642 more discs that may hold projects haven't been searched yet.

The set also has 30 files from another program that uses SuperCard's `MDOC` file type, with creator `MC3D`. scriptview must not take them for projects.

### Classic scripts

There are 51,161 distinct compiled AppleScripts from before Mac OS X: about 48,400 from about 3,570 discmaster discs, and about 3,900 from 63 localized disc images on archive.org. 50,580 have the header of AppleScript 1.1 and later, 489 of AppleScript 1.0, and 28 of the beta. These scripts test the readers of classic files and text encodings. The scripts on Apple's own discs are a smaller set with references of their own: see [Classic scripts](applescript/testing.md#classic-scripts).

### Disk images

[Disk images](disk-images.md) lists the real disks that scriptview was tested on: a System 7.1 hard disk, a Mac OS 9.1 hard disk, a Japanese Mac OS 8.6 install CD, and others.

## Every file reads

Each file must print in under 15 seconds, with no crash and no error. With the current build:

- All 38,879 stacks print their scripts in 1 minute 20 seconds, and their field text in 57 seconds.
- All 876 projects print their scripts in 7 seconds, and their field text in 9.
- All 51,161 classic scripts print, in about 3 minutes when run 8 at a time, each with its own 15-second limit.

The sweep over the classic scripts used to find 4 run-only scripts, of 570 to 700 KB, that took 20 seconds or ran for hours. The syntax tree that scriptview rebuilds from a run-only script's bytecode had 16-bit node IDs, like the trees in files, and these scripts need more than 32,767. Their IDs wrapped around, and the tree tangled. With 32-bit IDs, each prints in a tenth of a second.

## Stacks

A stack repeats some of its structure, and real stacks give a few references:

- The number of cards in a stack's header matches the cards that scriptview finds in 28,627 of the 28,683 HyperCard 2.x stacks without private access. In 9 of the other 56, the file is whole and holds fewer cards than its header says. The other 47 are damaged or truncated, and have lost cards. Before scriptview read on past damage, 79 didn't match. In HyperCard IIGS stacks, it matches in 721 of the 724. The other three are a truncated file and two private-access stacks.
- 12 stacks came with listings of their scripts that tools such as Script Stripper wrote out, with HyperCard's indentation. scriptview's indentation matches 7,444 of the 7,453 lines. One more stack came with a listing, but its listing isn't indented, so it isn't used.
- Of 11 stacks with private access, which scrambles part of the header, 10 print their scripts, and the other has none.
- 77 stacks hold 376 scripts in AppleScript. All of them decompile.
- The source code of stackimport, an open-source converter of HyperCard 2.x stacks, agrees with scriptview on every structure that both read.

## Projects

A SuperCard project is full of old memory, so the test is whether scriptview finds every script without picking up the old pieces of scripts. The reference is a separate, simpler tool: a Python script, outside the repository, that walks the block table, and takes every record in a block that looks like a script, without following the rules for windows, cards, and objects. It finds 49,294 distinct scripts that are ASCII and have a handler:

- scriptview prints all but 97 of them. 48 of the 97 are menu scripts that SuperCard copied into the project's block, which may be left over from older menus.
- scriptview prints 412 such scripts that the simpler tool misses.
- scriptview names the object of 97.6% of the objects' scripts. It prints the rest under the card or background, without naming the object.

Other checks on projects:

- Of the 41,408 protected scripts, 41,401 decode.
- Of 195,725 records of field text, 2,606 are empty, and in all the rest, the style runs fit inside the text. Every card's record of its background's field text reads.
- scriptview's header check takes none of the 117,881 stacks, classic scripts, and GitHub scripts in the other corpora for a project, and rejects all 30 `MC3D` files.

## Disk images

There are synthetic tests for each container: HFS and HFS+ volumes, built by `internal/hfs/hfstest`, inside a partition map, a GUID partition table, and Disk Copy 4.2, Disk Copy 6, and `.dmg` images. One test checks the back-references of ADC, the compression that Disk Copy 6 uses. No real compressed `.dmg` has been tested yet.

## Text encodings

[Text encodings](text-encodings.md#detection) gives how many stacks and scripts read in each encoding. The rules were checked on samples of each result: the stacks and scripts that read in an encoding other than Mac Roman, the ones that a change of rules turned away, and the scripts from localized discs, whose paths give their language.

## Comparing builds

Before a change is committed, scriptview runs on the corpora with the change and without it, and every difference in the output is read. A change should change only what it means to. Some examples:

- Reading folders: 1,000 files, across three commands, print the same as in 0.4.0, with the same exit codes.
- Reading disk images: 700 files print the same as in 0.4.0.
- Adding SuperCard: 3,000 stacks and scripts print the same as before.
- Adding SuperCard's field text: the scripts of 5 projects changed, all in object IDs above 65,535, which scriptview had cut to 16 bits. Field text had first counted toward the guess at the encoding, and it changed the encoding of 5 more projects for the worse. So the guess uses only scripts and names.
- Bounding the resource fork reader: every stack and project prints the same as before.
- Checking disk signatures more closely: 9 classic scripts that have `H+` or `BD` at offset 1024 by chance, which scriptview had taken for disk images since it started reading disk images, print the same as in 0.4.0 again.
- Widening the IDs of rebuilt syntax trees: 133 run-only classic scripts changed, all of them large enough to have run out of 16-bit IDs. Their old output had 282 damage markers, and the new output has none. No GitHub script changed, in either `decompile` or `bytecode`.
- Reading past damaged blocks: 61 stacks changed, all of them damaged. Their output has every script line it had before, and more, and their field text only gained text and better labels.

## Fuzzing

The Go fuzzer feeds each reader damaged files, built from the synthetic test files. A reader may fail on them, but must not crash or hang:

- `FuzzParse` in `internal/hypercard`, for stacks. It found a crash on a damaged first block.
- `FuzzParse` in `internal/supercard`, for projects. It sets the length in the header to the file's length, so that damaged projects get past the header.
- `FuzzOpen` and `FuzzNDIF` in `internal/hfs`, for volumes and Disk Copy 6 images.
- `FuzzUnwrap` in `internal/classic`, for MacBinary, BinHex, AppleSingle, AppleDouble, and resource forks. It found a resource fork whose types all claimed the same resources, which made the reader list each resource thousands of times. A damaged fork could have made that billions.

To run one, for example for a few minutes:

```
go test ./internal/hfs -run '^$' -fuzz FuzzOpen -fuzztime 5m -fuzzminimizetime 3s
```

Without `-fuzzminimizetime`, the fuzzer can spend minutes making each new input smaller, and look stuck.

## Known gaps

- Real compressed `.dmg` images haven't been tested.
- scriptview doesn't read the tokenized scripts of SuperCard 1 standalones.
- 28 projects in the corpus were saved by several versions of SuperCard. Comparing each version's output would check the rules for each format, but it hasn't been done.
