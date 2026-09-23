package scpt

import "fmt"

// objectAlias builds a reference from MakeObjectAlias's operands.
func (bh *bcHandler) objectAlias(in instr, stack *[]stackVal) (int16, error) {
	b := bh.b
	pop := func() (stackVal, error) {
		s := *stack
		if len(s) == 0 {
			return stackVal{}, errUnsupported{in, "stack underflow"}
		}
		v := s[len(s)-1]
		*stack = s[:len(s)-1]
		return v, nil
	}
	kind := in.args[0]
	switch kind {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11:
	default:
		return 0, errUnsupported{in, fmt.Sprintf("alias kind %d", kind)}
	}
	if kind == 5 { // container whose test
		test, err := pop()
		if err != nil {
			return 0, err
		}
		container, err := pop()
		if err != nil {
			return 0, err
		}
		id := b.node('6', container.id, test.id)
		if t := b.out.nodes[test.id]; len(t.children) > 0 && b.out.nodes[t.children[0]].typ == 'g' {
			b.out.nodes[id] = nodeRec{typ: '6', flags: flagAltSyntax, children: []int16{container.id, test.id}} // where it > 1
		}
		return id, nil
	}
	var idx, idx2, form stackVal
	var err error
	if kind == 4 { // [container, class, value, key form]
		if form, err = pop(); err != nil {
			return 0, err
		}
	}
	switch kind {
	case 3, 4:
		if idx, err = pop(); err != nil {
			return 0, err
		}
	case 6:
		if idx2, err = pop(); err != nil {
			return 0, err
		}
		if idx, err = pop(); err != nil {
			return 0, err
		}
	}
	var cls stackVal
	if kind != 9 && kind != 10 && kind != 11 {
		if cls, err = pop(); err != nil {
			return 0, err
		}
	}
	container, err := pop()
	if err != nil {
		return 0, err
	}
	if container.gotten {
		if cn := b.out.nodes[container.id]; cn.typ == 'n' || cn.typ == '1' {
			container.id = b.node('e', container.id) // item 1 of (get selection)
		}
	}
	var part int16
	switch kind {
	case 0: // property
		part = cls.id
	case 1: // every class
		part = b.node('2', bh.keyOf(cls.id))
	case 2: // some class
		part = b.node('3', bh.keyOf(cls.id))
	case 3: // class index
		part = b.node('4', bh.keyOf(cls.id), idx.id)
	case 4: // class named/id value
		code := "name"
		if f := b.out.nodes[form.id]; len(f.children) > 0 && b.out.osCode[f.children[0]] != "" {
			code = b.out.osCode[f.children[0]]
		}
		part = b.node('5', bh.keyOf(cls.id), idx.id, b.termRef(fasCode{kind: osKindCode, code: code}))
	case 6: // class from thru to
		n := b.node('7', bh.keyOf(cls.id), idx.id, idx2.id)
		b.out.nodes[n] = nodeRec{typ: '7', flags: flagThe, children: []int16{bh.keyOf(cls.id), idx.id, idx2.id}}
		part = n
	case 7, 8: // [class] before/after container
		part = b.node(map[int]byte{7: '8', 8: '9'}[kind], bh.keyOf(cls.id))
	case 9:
		part = b.node(':')
	case 10:
		part = b.node(';')
	case 11:
		part = b.node('<')
	}
	if container.it {
		return part, nil // implicit container: the tell target or whose's it
	}
	if cn := b.out.nodes[container.id]; (kind == 3 || kind == 4) && cn.typ == 'f' {
		return part, nil // POSIX file "/x", not … of me
	}
	if cn := b.out.nodes[container.id]; kind == 0 && cn.typ == 'f' {
		if p := b.out.nodes[part]; p.typ == 'o' {
			return part, nil // a script property of me: just its name
		}
	}
	id := b.node('n', part, container.id)
	if (kind == 0 || kind == 3) && bh.isObjCTarget(container.id) {
		// current application's NSString, x's |length|, current
		// application's class "NSEvent": the ASObjC idiom.
		b.out.nodes[id] = nodeRec{typ: 'n', flags: flagPossessive, children: []int16{part, container.id}}
	}
	return id, nil
}

// popArgs pops a count and that many values.
func popArgs(in instr, stack *[]stackVal) ([]stackVal, error) {
	s := *stack
	if len(s) == 0 {
		return nil, errUnsupported{in, "missing argument count"}
	}
	n := s[len(s)-1]
	if n.count < 0 || n.count > len(s)-1 {
		return nil, errUnsupported{in, "bad argument count"}
	}
	args := append([]stackVal(nil), s[len(s)-1-n.count:len(s)-1]...)
	*stack = s[:len(s)-1-n.count]
	return args, nil
}

// positionalCall builds handler(args…) from PositionalMessageSend.
func (bh *bcHandler) positionalCall(in instr, stack *[]stackVal) (int16, error) {
	b := bh.b
	args, err := popArgs(in, stack)
	if err != nil {
		return 0, err
	}
	s := *stack
	if len(s) == 0 {
		return 0, errUnsupported{in, "missing call target"}
	}
	target := s[len(s)-1]
	*stack = s[:len(s)-1]
	ids := make([]int16, len(args))
	for i, a := range args {
		ids[i] = a.id
	}
	call := b.node('I', b.keyRef(bh.lit(in.args[0])), b.cons(ids), b.ref())
	if target.it {
		return call, nil
	}
	n := nodeRec{typ: 'n', children: []int16{call, target.id}, flags: flagPossessive}
	if b.out.nodes[target.id].typ == 'f' {
		n.flags = flagAltSyntax // my call()
	}
	id := b.node('n')
	b.out.nodes[id] = n
	return id, nil
}

// messageSend builds a command or labeled handler call from MessageSend:
// [target/direct, key, value, …, count].
func (bh *bcHandler) messageSend(in instr, stack *[]stackVal) (int16, error) {
	b := bh.b
	args, err := popArgs(in, stack)
	if err != nil {
		return 0, err
	}
	if len(args)%2 != 0 {
		return 0, errUnsupported{in, "odd parameter list"}
	}
	s := *stack
	if len(s) == 0 {
		return 0, errUnsupported{in, "missing direct parameter"}
	}
	direct := s[len(s)-1]
	*stack = s[:len(s)-1]
	var keys, values []int16
	for i := 0; i < len(args); i += 2 {
		keys = append(keys, bh.keyOf(args[i].id))
		values = append(values, b.stmtValue(args[i+1])) // a GetData here is an explicit get
	}
	name := bh.lit(in.args[0])
	directID := b.ref()
	if !direct.it {
		directID = b.stmtValue(direct) // log (get class of x)
	}
	labels := b.ref()
	if len(keys) > 0 {
		labels = b.cells(keys, values)
	}
	return b.node('I', b.keyRef(name), directID, labels), nil
}

// errorCommand builds `error msg number n …` from Error:
// [me, key, value, …, count, message].
func (bh *bcHandler) errorCommand(in instr, stack *[]stackVal) (int16, error) {
	b := bh.b
	s := *stack
	if len(s) == 0 {
		return 0, errUnsupported{in, "missing error message"}
	}
	msg := s[len(s)-1]
	*stack = s[:len(s)-1]
	args, err := popArgs(in, stack)
	if err != nil {
		return 0, err
	}
	s = *stack
	if len(s) > 0 {
		*stack = s[:len(s)-1] // the PushMe target
	}
	var keys, values []int16
	for i := 0; i+1 < len(args); i += 2 {
		keys = append(keys, bh.keyOf(args[i].id))
		values = append(values, args[i+1].id)
	}
	labels := b.ref()
	if len(keys) > 0 {
		labels = b.cells(keys, values)
	}
	msgID := b.ref()
	if !msg.undefined {
		msgID = msg.id
	}
	return b.node('R', b.ref(), msgID, labels), nil
}

// isObjCTarget reports whether id is `current application` or a reference
// through it, where Script Editor output uses possessive ('s) syntax.
func (bh *bcHandler) isObjCTarget(id int16) bool {
	b := bh.b
	for i := 0; i < 16; i++ {
		n, ok := b.out.nodes[id]
		if !ok {
			return false
		}
		switch n.typ {
		case 'm', '1':
			return b.out.osCode[child0(n)] == "cura"
		case 'n':
			id = child(n, 1)
		default:
			return false
		}
	}
	return false
}
