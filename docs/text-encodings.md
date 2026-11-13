# Text encodings

Classic Mac OS had no single text encoding. Each script system, such as Roman, Japanese, or Cyrillic, had its own, and text took the encoding of the font it was set in. Neither a compiled script nor a HyperCard stack records which encoding its text uses, so scriptview works it out from the text, and from a stack's fonts.

## The encodings

scriptview decodes these encodings, with Apple's mapping tables from unicode.org:

- Mac Roman, for English and the Western European languages. Most classic files use it.
- Mac Japanese, Shift JIS with Apple's additions: circled numbers, Roman numerals, and other symbols from `0x8540` to `0x886D`, vertical forms from `0xEB41` to `0xED96`, and the one-byte characters `0x80` (backslash), `0xA0` (no-break space), `0xFD` (©), `0xFE` (™), and `0xFF` (…). It lacks the IBM and NEC rows that Windows code page 932 adds.
- Mac Chinese Simplified (GB 2312), Mac Chinese Traditional (Big5), and Mac Korean (KS C 5601, with about 1,100 extra symbols).
- Mac Central European, Mac Cyrillic, Mac Greek, Mac Hebrew, Mac Arabic, Mac Farsi, and Mac Thai.
- Mac Turkish, Mac Icelandic, Mac Croatian, and Mac Romanian, variants of Mac Roman that change a few letters.

`tools/genmactext` makes the tables from Apple's files. Some of Apple's entries map one byte pair to several characters, with a hint character from U+F860 to U+F87F, and the Arabic and Hebrew tables mark some characters with a direction. scriptview drops the hints and directions, and keeps the characters.

Bytes `0x00` to `0x7F` are ASCII in every encoding. Japanese fonts draw `0x5C` as ¥, but scriptview prints a backslash, since scripts use it as one.

## Detection

A file's text is Mac Roman unless there's evidence for another encoding. The evidence comes from two places.

First, a stack's font table. A font's ID tells its script system: IDs from 16384 to 32767 belong to the non-Roman scripts, 512 IDs to a script, starting with Japanese at 16384. Osaka, the Japanese system font, is 16384. Utilities that resolved font ID conflicts also put Roman fonts in that range, so scriptview trusts an ID only together with its name: a known name, such as Osaka or Beijing, or a name that isn't ASCII and decodes in the script's encoding, such as 細明朝体. A font hint makes its encoding a candidate. It doesn't decide alone, because a stack can list a font that none of its text uses.

Second, the text itself. scriptview decodes all of a file's strings in each candidate encoding, and counts the characters that show the encoding is right, and the ones that show it's wrong:

- Japanese counts kana. Kanji don't count, because Mac Roman's accented letters are Shift JIS lead bytes: "Frédéric" reads as "Fr仕屍ic".
- Korean counts hangul, and Chinese counts ideographs next to other wide characters.
- Against any of these, a half-width katakana letter alone next to an ASCII letter or digit. Mac Roman's curly quotes and dashes are half-width katakana in Shift JIS, so "haven’t" reads as "havenﾕt". HyperTalk's continuation character, `0xC2` at the end of a line, is an exception: it's ﾂ in Shift JIS, and Japanese scripts use it too.
- Against them too, an ideograph inside an ASCII word, as in "Ger閣etechnik" for "Gerätetechnik".

Japanese is always a candidate, because kana are evidence enough. What else can win depends on whether the file lists its fonts.

A stack lists them, so only its fonts' encodings are other candidates, and a stack without a font table, such as one from HyperCard 1.x, is read as Japanese or Mac Roman. Guessing from a stack's text alone fails: stacks set text in symbol fonts, and decorate with rows such as "∞∞∞", which read as hangul or Cyrillic.

A compiled script doesn't list its fonts, so scriptview weighs every encoding against Mac Roman, and picks one only when it fits clearly better:

- Kana, hangul, and the common hanzi of GB 2312 and Big5 count for their encodings. Chinese loses a point for a space between two wide characters, since Korean separates words with spaces and Chinese doesn't.
- In a single-byte encoding, a letter in a word counts for it, and a symbol touching a letter counts against it, as in "konuﬂmay›", the Mac Roman reading of Turkish "konuşmayı". So do a letter from another alphabet in mid-word, a capital after a small letter, and in the Latin encodings, a run of four accented letters, as in "ÑËÒÔÎÂÈ" for Russian "Дисплей".
- Single-byte encodings compete with Mac Roman only on the characters where their readings differ, and must win most of them. Western text often has a stray symbol that another encoding reads as a letter, but not many.
- A repeated character, as in "•••", counts for nothing, and a string with control characters is data, not text.

Greek and Thai don't take part: Greek reads the symbols in Western scripts, such as π, as letters, and Thai's letters fill most of the high bytes. Arabic and Hebrew have right-to-left copies of ASCII punctuation, which count as symbols.

A string that doesn't decode in the chosen encoding reads as Mac Roman. A stack often mixes the two: text in a Roman font beside Japanese text. Without a font hint, Japanese still needs nine in ten strings with high bytes to decode, and in compiled scripts, AppleScript's raw code syntax, such as `«class cfol»`, rules Japanese out: Mac Roman's « pairs with the next byte in Shift JIS.

Of 36,582 stacks from software CDs, scriptview reads 4,438 as Japanese, 24 as Chinese Simplified, 5 as Korean, 2 as Chinese Traditional, and 2 as Cyrillic. Samples of each, and of the stacks that the rules turned away, found no misreadings.

Of about 50,000 classic compiled scripts, the weighing reads about 960 as Chinese, 310 as Korean, 310 as Turkish, 120 as Cyrillic, 100 more as Japanese, 24 as Arabic, and 13 as Hebrew, which Mac Roman garbled before. Samples found a few Western scripts misread, among them one with "π" in a file name.
