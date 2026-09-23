try
	beep
on error number -128
	beep 2
end try
try
	beep
on error "x" number n from f partial result pr to t
	beep 3
end try
display dialog "a" buttons {"OK"} default button 1 with icon note
