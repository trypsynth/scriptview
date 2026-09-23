global config
property p : 1
set config to 2
on f()
	set y to config + p
	return result
end f
script S
	property q : 1
	on g()
		return q + config
	end g
end script
