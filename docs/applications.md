# Applications

When you write `tell application "Safari"`, the compiler doesn't store the name `Safari`. It finds the app and stores an alias record, a small binary record that describes where the file is. `osadecompile` reads the alias back and decides how to show the app.

## The alias record

An alias record is a classic Mac OS format. scriptview uses these parts of it:

- The file name, such as `Safari.app`, as a Pascal string at offset 50.
- A list of tagged items, starting at offset 150. Each item has a 16-bit tag, a 16-bit length, and the data. Tag -1 ends the list.

| Tag | Data |
| --- | --- |
| 2 | The HFS path, with colons, such as `Macintosh HD:Applications:Safari.app` |
| 14 | The file name in UTF-16 |
| 15 | The volume name in UTF-16 |
| 18 | The POSIX path, relative to the volume |
| 19 | The volume's mount point, such as `/` |

The alias also keeps the file's ID number on the disk, which matters in [Renamed apps](#renamed-apps).

The same record is used for `alias "…"` literals. `osadecompile` prints these as an HFS path with the startup disk's name in front, such as `alias "Macintosh HD:Users:me:Desktop:"`. scriptview takes the path from tag 18 and puts `Macintosh HD:` in front. On a Mac with a renamed startup disk, `osadecompile` prints that disk's name instead.

## How an app is named

`osadecompile` names an app by its display name when it can find the app, and by its file name when it can't:

```applescript
tell application "Safari"
tell application "Firefox.app"
```

The first line found Safari. The second looked for Firefox and didn't find it. The same script prints differently on different Macs.

scriptview names apps this way:

- Apps that ship with macOS always print by name. `tools/genterms` saves the list.
- On a Mac, scriptview checks whether the app is at the path in the alias. If it isn't, scriptview asks Spotlight (`mdfind`) for an app with that file name, because macOS finds apps that have moved. If either check succeeds, the app prints by name.
- On other systems, other apps print by file name.

### Renamed apps

Apps that were renamed need special handling:

- `System Preferences` prints as `System Settings`, and `iCal` prints as `Calendar`, because macOS knows the old names.
- `AppleScript Editor`, Script Editor's name in Mac OS X 10.6 to 10.9, gets Script Editor's terms. `osadecompile` prints it as `Script Editor` when it can find the app by its bundle ID. scriptview prints the alias's file name, `AppleScript Editor.app`, because the result depends on what's installed.
- `Address Book` usually prints as `Contacts`. Two scripts in the test corpus printed as `Address Book.app` instead. Their alias records matched the others except for the file ID. The likely explanation is that macOS tried the ID first, and on the test Mac that ID belonged to another file. This can't be predicted without the same disk, so scriptview doesn't try.

## Apps by URL

A script can target a web service: `tell application "http://example.com/soap"`. That application specifier stores an `aprl` URL instead of an alias. These apps have their own small dictionary, with `call soap` and `call xmlrpc`. `file://` URLs appear here too, and print by the app's name.

## Apps by bundle ID

`application id "com.apple.Safari"` stores the bundle ID as text, without an alias. scriptview maps bundle IDs and the older four-character creator codes to apps with the list from `tools/genterms`.
