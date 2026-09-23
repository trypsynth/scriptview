# hash comment
#
use AppleScript version "2.4"
use scripting additions

property items_ : [1, 2, 3]

to pushItem onto L as list at i as integer : 0 given value:v : missing value
	return L
end pushItem

on make2 from a to b
	local x, y
	set x to text from a to b of "abcdefgh"
	set y to characters a thru b of "abcdefgh"
	return {x, y}
end make2

set L to {1, 2, 3}
set f to 1st item of L
set g to 2nd item of L
set h to front item of L
tell application "Finder"
	set w to window named "x"
	if (exists of window 1) then beep
	if window 1 exists then beep
end tell
with timeout of 1 second
	set z to 2
end timeout
tell me to pushItem onto L at 1 given value:3
script s
	property p : 1
	on beep
		continue beep
	end beep
end script
set q to "a" & ¬
	"b"
if q is "ab" then ¬
	set q to "c"
tell application "System Events" to ¬
	set n to name of every process
