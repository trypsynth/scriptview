use framework "Foundation"
use scripting additions
property L : {1}
property i : 1
on f(x)
	return x
end f
-- not before a command, and tell targets
if not exists item 2 of L then beep
if not f(1) then beep
set r to a reference to first item of L whose it > 0
repeat with t in every item of L whose it > 0
end repeat
tell make new script object
end tell
tell application "Finder" to tell first item of home whose name is "x"
end tell
tell application "Finder" to make new folder at front
tell application "Finder" to make new folder at back
-- my in operators and ranges
if 1 is not in my L then beep
if my f(1) is in L then beep
set a to 1 + my i
set a to 2 * my f(1)
set t to text my i thru -1 of "abc"
set t to text (my f(1)) thru -1 of "abc"
-- escapes inside pipes
set rec to {|\\n|:1, |\\|:2, |\||:3, |app|:4}
-- reals: 12 decimal places
set reals to {3.141592653589793, 0.333333333333333, 9.99999999999999, 1.23456789012345E+15}
-- Objective-C calls
set d to current application's NSMutableArray's new()
d's addObject:do shell script "echo hi"
tell current application's NSString's stringWithString:"a"
end tell
-- and/or chains need no parentheses
if 1 = 1 and 2 = 2 and 3 = 3 or 4 = 4 or 5 = 5 then beep
