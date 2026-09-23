set p to POSIX path of (path to desktop as alias)
set q to (path to desktop as text) & "::"
set r to POSIX path of ((path to me as text) & "::")
set s to quoted form of POSIX path of p
set AppleScript's text item delimiters to ","
set t to text items of "a,b"
set u to (t as string)
set v to "x" & (count of t) as string
set w to (items 2 thru -1 of t) as text
set x to name of (info for p)
tell application "Finder" to set y to (POSIX path of (container of (path to me) as alias))
