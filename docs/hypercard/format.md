# Stack format

This page describes a HyperCard stack file as scriptview reads it. Apple never published the format. The description comes from reading stacks, checked against two earlier readings of the format:

- Rebecca Bettencourt's format notes (`HC_FILE_FORMAT_2010.TXT`, kreativekorp.com), based on Tyler Vano's work.
- Uli Kusterer's [stackimport](https://github.com/uliwitness/stackimport), which reads the same format, including the pictures that scriptview skips. It handles only HyperCard 2.x.

scriptview was tested on these stacks:

- 29 stacks from Apple's 1993 and 1994 discs, among them HyperCard's own Home stack and Apple's AppleScript developer tools.
- 4,651 stacks from software CDs, through discmaster.textfiles.com and the HyperCard Online collection on archive.org: 3,227 from HyperCard 2.x, 1,381 from HyperCard 1.x, 38 from HyperCard IIGS, and 5 from Plus.
- 12 stacks that came with text listings of their scripts, which show how HyperCard indents. See [Scripts and indentation](#scripts-and-indentation).

All numbers are big-endian. Text is Mac Roman, with a carriage return at the end of each line.

## Blocks

A stack file is a chain of blocks with nothing between them. Each block starts with a 16-byte header:

- 4 bytes: the block's size, header included.
- 4 bytes: the block's type, four characters such as `CARD`.
- 4 bytes: the block's ID, a signed number. Blocks refer to each other by type and ID.
- 4 bytes: unused in HyperCard 2.x. HyperCard 1.x puts a field here that 2.x moved later, which shifts the rest of a background's or card's header. See [HyperCard 1.x](#hypercard-1x).

The first block is always `STAK`, and the last is `TAIL`. The 29 stacks hold the following types. scriptview reads only `STAK`, `LIST`, `PAGE`, `BKGD`, and `CARD`. The roles of the others come from other readers of the format, such as stackimport, not from scriptview's own reading.

- `STAK`: the stack itself, with its settings and script.
- `MAST`: an index from block IDs to file offsets.
- `LIST` and `PAGE`: the order of the cards.
- `BKGD`: a background.
- `CARD`: a card.
- `BMAP`: a card's or background's picture.
- `STBL` and `FTBL`: the text styles and the fonts that field text uses.
- `PRNT`, `PRST`, and `PRFT`: print settings and report templates.
- `FREE`: unused space.

## The stack block

The `STAK` block's fields come at fixed offsets from the start of the block:

- At 16: the format version. 10 is HyperCard 2.x. The 1.x stacks have 8.
- At 36: the number of backgrounds.
- At 40: the ID of the first background.
- At 44: the number of cards.
- At 48: the ID of the first card.
- At 52: the ID of the `LIST` block.

The stack script starts at offset 1536, as a string that ends with a NUL byte.

## Card order

The cards' order in the file isn't their order in the stack. The `LIST` block gives the order through a set of `PAGE` blocks:

- At 16: the number of pages.
- At 28: the size of each card entry in a page, 2 bytes.
- From 48: one 6-byte item for each page, the page's ID and the number of cards on it.

A `PAGE` block lists its cards from offset 24, one entry each. An entry starts with the card's ID. The rest of the entry holds data that speeds up `find`, which scriptview doesn't need. Its size varies from stack to stack: the 29 stacks have entries from 12 to 132 bytes long.

## Backgrounds and cards

Backgrounds and cards have the same layout after a short header. A background's header holds:

- At 16: the ID of its picture, a `BMAP` block, or 0 for none.
- At 20: flags, 2 bytes.
- At 24: the number of cards that use the background.
- At 28 and 32: the IDs of the next and previous backgrounds. The backgrounds form a ring, starting from the one the stack block names.

A card's header holds:

- At 16: the ID of its picture.
- At 20: flags, 2 bytes.
- At 32: the ID of the `PAGE` block that lists the card.
- At 36: the ID of its background.

Then both continue, at 36 in a background and at 40 in a card:

- The number of parts, 2 bytes.
- The ID that the next new part will get, 2 bytes.
- The size of the part list, 4 bytes.
- The number of field text entries, 2 bytes.
- The size of the field text list, 4 bytes.
- The part list.
- The field text list.
- The name, as a string that ends with a NUL byte.
- The script, the same way.

### Parts

A part is a button or a field. Each part record holds:

- At 0: the record's size, 2 bytes.
- At 2: the part's ID, 2 bytes. IDs are unique within a card or background, not within the stack.
- At 4: the kind, 1 byte: 1 for a button, 2 for a field.
- From 5: flags, the part's rectangle, and its text style.
- At 30: the name, as a string that ends with a NUL byte.
- After the name and one pad byte: the script, the same way.

### Field text

A field's text lives with the card it's on, not with the field. Each entry in a card's field text list holds:

- The part's ID, 2 bytes, signed. A negative ID is one of the card's own parts. A positive ID is a part of the card's background: the text that a background field shows on this card.
- The length of what follows, 2 bytes.
- The text. It starts with a 0 byte for plain text. For styled text, it starts with a style table whose 2-byte length has the high bit set: the length without that bit includes the 2 bytes themselves, and each 4-byte run gives an offset in the text and a style ID from the `STBL` block.

Entries are padded to an even length. A background's own field text list holds the text of background fields that show the same text on every card.

Buttons have entries too. A card part's entry is the button's text. A background button's entry on a card isn't text: it's `1` when the button is highlighted on that card, for buttons that don't share their highlight across cards.

## Protected stacks

A stack with private access needs a password to open. Bit `0x2000` of the 16-bit flags word at offset 76 of the `STAK` block marks it. In such a stack, HyperCard scrambles the `STAK` block's header from offset 36 on: the numbers of backgrounds and cards, and the IDs of the first background, the first card, and the `LIST` block. The rest of the file isn't scrambled. scriptview reads the backgrounds and cards in file order when the header doesn't lead to them. Of the 11 protected stacks in the test set, the 10 that have scripts print them.

The sum of the first 1,536 bytes of the `STAK` block, read as big-endian 32-bit words, is 0 in an ordinary stack, and not in a protected one.

## Other stack formats

These files also start with a `STAK` block:

- HyperCard IIGS, the Apple IIGS version, stores its numbers little-endian. Its scripts are plain text in the same block types. scriptview doesn't read these stacks yet.
- Plus, a cross-platform HyperCard clone, writes the format version `0x80000008`. Plus stacks read like HyperCard 2.x stacks. Plus stores its scripts already indented, which scriptview replaces with its own indentation.

## Text encoding

Text is Mac Roman, unless the stack is Japanese. Like compiled AppleScript, a stack doesn't record its encoding. scriptview reads a stack as Mac Japanese, a variant of Shift JIS, when all its non-ASCII text decodes as Shift JIS and holds several kana. Kanji alone aren't enough: an accented Mac Roman letter followed by certain bytes forms a valid pair, so "Frédéric" reads in Shift JIS as "Fr仕屍ic". 455 of the 4,651 stacks in the test set read as Japanese.

## HyperCard 1.x

HyperCard 1.x stacks use the same blocks with three differences:

- A background's or card's header has its picture ID at 12, where 2.x has an unused field, so every later field comes 4 bytes earlier than in 2.x.
- The `LIST` block's page count and entry size, and the start of each page's entries, also come 4 bytes earlier.
- Field text has no length and no styles. Each entry is the part's ID, then the text, ending with a NUL byte.

## The resource fork

A stack's resource fork holds what HyperCard keeps outside the stack format: icons, cursors, sounds, fonts, and pictures, and the externals that scripts call. An external is compiled code in an `XCMD` resource, for a command, or an `XFCN` resource, for a function. Scripts call it by the resource's name, as they would call a handler, so a script that calls one looks as if it calls a handler that doesn't exist. scriptview lists the externals' names at the top of its output.

The Apple Event XCMDs stack, for example, carries `AESend` and 18 others, which let HyperCard 2.1 scripts send Apple events before HyperCard could run AppleScript.

## Scripts and indentation

Scripts are stored without indentation: HyperCard indents a script each time it shows it, and removes the indentation when it saves. Of the 2,609 scripts in Apple's 29 stacks, not one stored line starts with a space or a tab, although field text in the same stacks often does.

The indentation rules come from 12 stacks that their authors distributed with listings of their scripts, written out by tools such as Script Stripper, which save `the script` of each object as HyperCard hands it out. scriptview's indentation matches 7,444 of the 7,453 lines in those listings. The rules:

- Each level is two spaces.
- `on` and `function` start a handler at the left margin, and open a level. A new handler closes anything that an earlier one left open.
- `end` with the handler's name, as written, closes the handler and any block left open in it. In `on f-- x`, the name is `f--`, so `end f` doesn't close it. `end if` and `end repeat` close one block.
- `repeat` opens a level.
- `if … then` at the end of a line opens a level. `if … then` followed by a statement opens none.
- `then` can start the next line, after `if c`. It lines up with the `if`, and opens a level when nothing follows it.
- `else` lines up with its `if`. After `if … then x` on one line, a bare `else` on the next line belongs to that `if` and opens a level, which gives cascades:

  ```
  if a then x
  else
    if b then y
    else z
  end if
  ```

- `else` followed by a statement ends the `if`. `else repeat` ends the `if` and opens a loop.
- A line continued with `¬` stays at the level of the line it continues.
- Blank lines and comment lines take the current level.

HyperTalk strings have no escape characters, so `then` or `--` inside quotes never counts.
