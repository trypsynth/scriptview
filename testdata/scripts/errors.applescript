try
	error "bad" number 500 partial result {1} from "src" to integer
on error errMsg number errNum from errFrom partial result pr to errTo
	log errMsg
end try
try
	1 / 0
on error number -2701
	log "div"
end try
considering case but ignoring white space
	set a to "A" = "a"
end considering
set x to missing value
if x is not missing value then log "x"
if class of x is not text then log "not text"
set {a, b} to {1, 2}
set {a, b} to {b, a}
set l to {1, 2, 3}
copy 9 to item 1 of l
set r to a reference to item 2 of l
set contents of r to 7
set h to hours of (current date)
set ts to time string of (current date)
set wd to weekday of (current date)
tell application "System Events" to keystroke "a" using {command down, shift down}
display notification "done" with title "T" subtitle "S" sound name "Glass"
