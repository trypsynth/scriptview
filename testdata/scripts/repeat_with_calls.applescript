use framework "Foundation"
repeat with anItem in L
	set s to x's objectForKey:"a"
	set t to anItem's objectForKey:"b"
	anItem's objectForKey:"c"
	set u to anItem's foo:1 bar:2
end repeat
repeat with i from 1 to 3
	set s to i's objectForKey:"d"
end repeat
set t to anItem's objectForKey:"e"
repeat with i from 1 to 3
	if true then
		set s to x's objectForKey:"a"
	end if
	set s to my foo:1
	set s to foo(x's objectForKey:"b")
	set s to (x's objectForKey:"c")'s bar:2
	repeat 2 times
		set s to x's objectForKey:"d"
	end repeat
end repeat
repeat while true
	set s to x's objectForKey:"f"
end repeat
repeat until true
	set s to x's objectForKey:"g"
end repeat
repeat
	set s to x's objectForKey:"h"
end repeat
on h(L)
	repeat with i in L
		return i's objectForKey:"a"
	end repeat
end h
