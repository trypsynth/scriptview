# Applications

When you write `tell application "Safari"`, the compiler doesn't store the name "Safari". It finds the app and stores an alias record: a small binary record that describes where the file is. `osadecompile` reads the alias back and decides how to show the app.

## The alias record

An alias record is an old Mac OS format. The parts we use:

- The file name, such as `Safari.app`, as a Pascal string at offset 50.
- A list of tagged items that starts at offset 150. Each item has a 16-bit tag, a 16-bit length and the data. Tag -1 ends the list.

| Tag | Data |
| --- | --- |
| 2 | The HFS path, with colons, such as `Macintosh HD:Applications:Safari.app` |
| 14 | The file name in UTF-16 |
| 15 | The volume name in UTF-16 |
| 18 | The POSIX path, relative to the volume |
| 19 | The volume's mount point, such as `/` |

The alias also keeps the file's ID number on the disk, which becomes important below.

The same record is used for `alias "..."` literals in a script. `osadecompile` prints those as an HFS path with the boot disk's name in front, such as `alias "Macintosh HD:Users:me:Desktop:"`. We take the path from tag 18 and put `Macintosh HD:` in front. That matched every script we tried, but on a Mac with a renamed disk, `osadecompile` would print that disk's name.

## How the app name is shown

`osadecompile` shows an app by its name when it can find the app, and by its file name when it can't:

```applescript
tell application "Safari"
tell application "Firefox.app"
```

The first one found Safari. The second one looked for Firefox and didn't find it. So the same script prints differently on different Macs.

What we do:

- Apps that ship with macOS always print by name. `tools/genterms` saves the list.
- On a Mac, we check whether the app is at the path in the alias. If it isn't, we ask Spotlight (`mdfind`) for an app with that file name, because macOS finds apps that have moved. If either works, we print the name.
- On other systems, other apps print as the file name.

Apps that were renamed need care:

- `System Preferences` prints as `System Settings`, and `iCal` prints as `Calendar`. macOS knows the old names.
- `Address Book` usually prints as `Contacts` too. But two scripts in our tests printed `Address Book.app`. Their alias records looked the same as the others except for the file ID. Our best guess is that macOS tried the ID first, and on our Mac that ID belonged to some other file. You can't predict that without the same disk, so we don't try.

## Apps by URL

A script can target a web service: `tell application "http://example.com/soap"`. That app specifier stores an `aprl` URL instead of an alias. These apps have their own small dictionary with `call soap` and `call xmlrpc`. `file://` URLs also show up here, and we print those by the app's name.

## Apps by bundle ID

`application id "com.apple.Safari"` stores the bundle ID as text, so it doesn't need an alias. We map bundle IDs and old four-character creator codes to apps with the list from `tools/genterms`.
