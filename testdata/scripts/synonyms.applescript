set a to 1
set b to 2
set e1 to a = b
set e2 to a is b
set e3 to a equals b
set e4 to a is equal to b
set n1 to a ≠ b
set n2 to a is not b
set n3 to a does not equal b
set n4 to a is not equal to b
set l1 to a < b
set l2 to a is less than b
set l3 to a comes before b
set g1 to a > b
set g2 to a is greater than b
set g3 to a comes after b
set le1 to a ≤ b
set le2 to a is less than or equal to b
set le3 to a is not greater than b
set ge1 to a ≥ b
set ge2 to a is greater than or equal to b
set ge3 to a is not less than b
set c1 to "ab" contains "a"
set c2 to "a" is in "ab"
set c3 to "a" is contained by "ab"
set c4 to "ab" does not contain "z"
set c5 to "z" is not in "ab"
set s1 to "ab" starts with "a"
set s2 to "ab" begins with "a"
set s3 to "ab" ends with "b"
set k1 to a as text
set x1 to 3 div 2
set x2 to 2 ^ 3
tell application "Finder"
	set w1 to every window whose name is "x"
	set w2 to windows where name is "x"
	set w3 to every window
	set w4 to windows
	set w5 to window 1
	set w6 to first window
	set w7 to last window
	set w8 to some window
	set w9 to window "x"
	set w10 to middle window
	set w11 to windows 1 thru 2
	set w12 to name of every window
end tell
