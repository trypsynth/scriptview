-- a comment
display dialog "Hi" buttons {"OK", "Cancel"} default button 1 with title "T" giving up after 5
set r to display dialog "Q" default answer ""
say "hello" without waiting until completion
set p to path to desktop
delay 1
log "x"

try
	error "boom" number 42
on error errMsg number errNum
	display alert errMsg
end try
(* block
comment *)
tell application "Finder" to activate
set t to text returned of r
