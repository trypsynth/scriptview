property p : 1
on a1()
	script Z
		on inner()
			return 1
		end inner
	end script
	return Z
end a1
on a2()
	script Z
		property q : 2
	end script
	return Z
end a2
on a3()
	script Z
		on inner()
			return p
		end inner
	end script
	return Z
end a3
on a4()
	set loc to 5
	script Z
		on inner()
			return loc
		end inner
	end script
	return Z
end a4
on a5()
	script Z
		property q : p
	end script
	return Z
end a5
