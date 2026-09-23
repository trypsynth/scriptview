property a : 1
property b : 2
on run {}
	tell application "Finder"
		set p to get name of startup disk
	end tell
	return p

end run
on f()
	return 1

end f
on g()
	beep
end g
