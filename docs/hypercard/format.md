# Stack format

This page describes a HyperCard stack file as scriptview reads it. Apple never published the format. The description comes from reading 29 stacks on Apple's 1993 and 1994 discs: 27 from HyperCard 2.x and 2 from HyperCard 1.x, among them HyperCard's own Home stack and Apple's AppleScript developer tools. Uli Kusterer's [stackimport](https://github.com/uliwitness/stackimport) reads the same format, including the pictures that scriptview skips.

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

## HyperCard 1.x

HyperCard 1.x stacks use the same blocks with three differences:

- A background's or card's header has its picture ID at 12, where 2.x has an unused field, so every later field comes 4 bytes earlier than in 2.x.
- The `LIST` block's page count and entry size, and the start of each page's entries, also come 4 bytes earlier.
- Field text has no length and no styles. Each entry is the part's ID, then the text, ending with a NUL byte.

## Scripts and indentation

Scripts are stored as typed, without indentation: HyperCard's script editor indents a script each time it opens it. scriptview indents them the same way, two spaces per level:

- `on` and `function` handlers, `repeat` loops, and `if … then` at the end of a line open a level.
- `else` lines up with its `if`. `else` followed by a statement on the same line ends the `if`.
- `if … then` followed by a statement on the same line opens nothing.
- A line continued with `¬` is indented one level deeper than the line it continues.
- `end` with a handler's name closes the handler, and any block left open inside it.

HyperTalk strings have no escape characters, so `then` or `--` inside quotes never counts. The two-space width and the depth of continued lines match HyperCard 2.x's editor as far as the stacks show. The exact rules of that editor aren't known.

Over the 2,609 scripts in the 29 stacks, every handler closes back at the left margin, which is a check that the rules agree with how the scripts were written.
