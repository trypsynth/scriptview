using terms from application "http://www.apple.com/placebo"
	tell application "http://example.com/x"
		set r to call soap {method name:"a", method namespace uri:"b", parameters:{}, SOAPAction:"c"}
		set s to call xmlrpc {method name:"a", parameters:{}}
	end tell
end using terms from
