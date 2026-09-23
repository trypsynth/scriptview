on run argv
	set r to fact(5)
	set s to joinText({"a", "b"}, ",")
	set t to area given width:2, height:3
	set u to rangeSum from 1 to 10
	return {r, s, t, u}
end run

on fact(n)
	if n ≤ 1 then return 1
	return n * (fact(n - 1))
end fact

on joinText(theList, delim)
	set {oldTID, AppleScript's text item delimiters} to {AppleScript's text item delimiters, delim}
	set out to theList as text
	set AppleScript's text item delimiters to oldTID
	return out
end joinText

on area given width:w, height:h
	return w * h
end area

on rangeSum from a to b
	set total to 0
	repeat with i from a to b
		set total to total + i
	end repeat
	return total
end rangeSum
