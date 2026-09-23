script V
	property parent : AppleScript
	on initWithFrame:frame
		continue initWithFrame:frame
	end initWithFrame:
end script
tell application "System Events"
	tell process "Finder"
		get name of any button of window 1
	end tell
end tell
