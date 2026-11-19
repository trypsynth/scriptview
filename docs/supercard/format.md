# SuperCard project format

SuperCard, from Silicon Beach Software in 1989 and later Aldus, Allegiant, and Solutions Etcetera, was a rival to HyperCard with color, several windows to a project, draw graphics, and its own menus. Its language, SuperTalk, is a superset of HyperTalk. Nobody published the file format. This page comes from reading the bytes of 876 projects and standalones, saved by every version from 1.0 to 4.5, and checking each rule against all of them. Where a rule is an inference, the page says so.

All numbers are big-endian. Offsets are from the start of the data fork, or of a block where the text says so.

## Files

A project keeps everything in its data fork, in every version. The resource fork, when there is one, holds only ordinary resources such as `vers` and `ckid`. SuperCard keeps its own resources, such as icons, cursors, sounds, and external commands, in the data fork too.

The file types are:

- `MDOC`: SuperCard 1 and 2. Authors could set their own creator, so the creator varies. Another program also uses the `MDOC` type, with creator `MC3D`, and its files fail the header check below.
- `MDO3`: SuperCard 3, creator `RUN3`.
- `MDOX`, `MDPX`, and `MDTX`: SuperCard 4.0 and 4.1 projects, Player projects, and trial projects.
- `MDOx`, `MDPx`, and `MDTx`: the same for SuperCard 4.5.

Since the types vary, scriptview goes by the header instead.

A standalone application, of type `APPL` with any creator, has its project in its data fork. In a 68k standalone, the data fork is an ordinary project. In a PowerPC or fat standalone, the application's code fragment (`Joy!peff`) sits at 0x400, the offset that its `cfrg` resource gives. The project is then the data fork's first 0x400 bytes, followed by its last length minus 0x400 bytes, where length is the project's length from the header. This held for all 189 PowerPC and fat standalones.

## Header

The first 0x400 bytes are the header:

- 0x00, 2 bytes: the format's major version. 1 for SuperCard 1 and 2, 2 for SuperCard 3 to 4.5.
- 0x02, 2 bytes: the minor version, which also tells a trial version's saves apart. 1.0 is SuperCard 1 and 2, and 1.1 the 2.x trial. 2.1 is SuperCard 3, 2.3 is 4.0 and 4.1, and 2.6 is 4.5, and one more in each case is that version's trial.
- 0x04, 4 bytes: the project's length, which equals the data fork's length except in PowerPC standalones.
- 0x08, 4 bytes: the free bytes. 0x0C, 4 bytes: the number of free extents, listed from 0x200 as offset and length pairs.
- 0x10, 4 bytes: the number of slots in the block table, a multiple of 128.
- 0x14, 4 bytes: the next unused block ID. 0x18 and 0x1C: the number of free IDs, and the first of them.

scriptview takes a file as a project when the major version is 1 or 2, the minor version is below 16, the length matches, the table size is a multiple of 128, and block 0 is a project block.

## Blocks

From 0x400, the block table has a 4-byte offset for each block ID. The blocks come after the table. Each block starts with a 12-byte header:

- 0, 4 bytes: its ID, the same as its slot.
- 4, 4 bytes: the length of the rest of the block.
- 8, 4 bytes: its allocated size, the header and the length rounded up to 64 bytes.

Its kind follows, in 2 bytes at 12, then 2 zero bytes. A slot is in use when its offset is a multiple of 64, points past the table, and points at a block with the slot's ID. A free slot holds the next free ID instead.

Past each block's length, in the free extents and in many fields that held pointers, is old memory from the Mac that saved the file. It often holds pieces of old scripts. So scriptview reads only within each block's length, and never scans the file for text.

The kinds are:

- 0: the project, always block 0, with the project's script.
- 1: four small lists, blocks 1 to 4, which scriptview doesn't need.
- 2: a card list, which gives a window's cards in order.
- 3: an index of SuperCard's own resources.
- 4: a window.
- 5: a menu.
- 6: a resource's data.
- 7: a card.
- 8: a background.

## Records

Most variable data is a record: a 4-byte length, then that many bytes, padded to an even length.

A script is a record that holds the text, in the Mac's system encoding with carriage returns between lines, then a NUL, then the script editor's state: 22 bytes in most files, 23 or 26 in some from SuperCard 4, ending in the editor window's rectangle. An empty script is a single carriage return.

Each kind of block keeps its own script at a fixed offset:

- In format 1.0: the project at 38, a menu at 42, a background at 48, a card at 52, and a window at 68.
- In format 1.1: 2 more in each case.
- In format 2: the project at 54, a menu at 58, a background at 64, a card at 68, and a window at 84.

## Windows and their cards

A window's block lists its card lists just before where its script would be if it had one list: at 60 in format 1.0, 62 in 1.1, and 76 in format 2. There are 2 bytes for the number of lists, then for each list, 2 bytes for its number of cards and 4 for its block ID. A list holds up to 100 cards, so a window with 830 cards has 9 lists. The window's script follows the lists, then its records, the last of which is its name. In format 2, the user properties can come before the name.

A card list has, from 14, 64 bytes for each card, in card order: the card's block ID, its background's block ID, the card's ID number, and the background's ID number, 4 bytes each. The ID numbers are the ones that scripts use, as in `card id 101`.

Each window points to a card list that exists in all but 8 of 5,440 windows. A card or background block that no window lists turns up in a few projects, and scriptview prints its scripts as well.

## Cards and backgrounds

A card's or a background's block has its script, then a record with its object list, then more records: for a card, the text of its background's fields on that card, and then for both, its user properties in format 2, and its name last.

The object list has a fixed record for each object, then the records that the objects point to, then the number of objects in 2 bytes. The fixed records are 82 bytes in format 1.0, 84 in 1.1, and 98 in format 2. In each:

- 0, 2 bytes: the type. Graphics are 100 to 108, buttons 200 to 223, and fields 300 to 303. Type 108 is a group.
- 8, 2 bytes: the object's ID.
- 58, 62, and 66, 4 bytes each: handles to the object's name, its data, and its script. The data is a field's text, a graphic's drawing, a polygon's points, or a group's objects.

The handles are memory addresses from when the file was saved, so their values mean nothing, but a handle that isn't 0 says that the object has that record. The records come in object order, and for each object in the order data, name, script. In format 1, this accounts for every record of all but a few cards: 114 of the scripts in format 1 files don't match up.

In format 2, an object can have more records, such as its user properties, whose handle is usually at 86, and sometimes at 72, 78, or 94. Not every value at those offsets is a handle, so scriptview tells them apart by trial: a script record is easy to recognize, by its NUL and the size and rectangle of the editor state after it, and a name has no NUL. For each object, it tries the most extra records first, and fewer until every script lands on a script handle and the records run out at the last object. This matches the records to their objects in all but 2,223 of the 92,670 objects with scripts. For the rest, scriptview prints the scripts under the card or background, without naming their object.

A group's data is an object list of its own, laid out the same way, and its objects are on the group's card or background.

## Menus

A menu's block has its script, then a record with its items, then its name last. The item list has a fixed record for each item, 26 bytes in format 1.0, 28 in 1.1, and 42 in format 2, then the records they point to, then the number of items in 2 bytes. An item's name handle is at 18 and its script handle at 22. In format 2 an item can have another record, such as its user properties. An item's script handles the `itemSelect` message.

## Protected and tokenized scripts

A protected script, from SuperCard 3 or 4, starts with `CE%` and five zero bytes. The rest of the record is XORed with 0xFD, either all of it or the text before its last carriage return, with the carriage return, the NUL, and the editor state left as they are.

Standalones made with SuperCard 1 can store their scripts tokenized, starting with the bytes 13 45 77 FF, then a pool of names and literals and a stream of tokens. They have no source text. Reading them would take SuperCard's table of tokens, from its runtime's code, so scriptview prints a comment in place of each one. 52 of the 876 files have tokenized scripts.

## Text encoding

Nothing in a project names its text encoding: it's the system encoding of the Mac that saved it. scriptview reads the text as Mac Roman unless the text reads better in another encoding, by the same rules as for compiled scripts. See [Text encodings](../text-encodings.md). A few standalones in the corpus are in Japanese.

## Compared with HyperCard

- HyperCard tags each block with a four-character type and finds blocks through a master table and its list and page blocks. SuperCard finds blocks only by ID, through one table, and gives each a number for its kind.
- A HyperCard part keeps its name and script inside its part record, as text that ends in a NUL. SuperCard keeps them in records after the fixed records, found through handles.
- HyperCard stacks hold little old memory. SuperCard projects are full of it, so a reader must follow the table and the lengths.
