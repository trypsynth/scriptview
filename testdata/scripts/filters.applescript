tell application "Finder"
	set a to every file whose name is "x"
	set b to every file that name is "x"
	set c to first file whose its name is "x"
	set d to first file that its name is "x"
	set e to files where name is "x"
end tell
