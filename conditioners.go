package gomodels

type Conditioner interface {
	Predicate() map[string]Value
	Next() (c Conditioner, isOr bool, isNot bool)
	And(q Conditioner) Conditioner
	AndNot(q Conditioner) Conditioner
	Or(q Conditioner) Conditioner
	OrNot(q Conditioner) Conditioner
}

type condChain struct {
	root Q
	next Conditioner
	or   bool
	not  bool
}

func (c condChain) Predicate() map[string]Value {
	return c.root.Predicate()
}

func (c condChain) Next() (Conditioner, bool, bool) {
	return c.next, c.or, c.not
}

func (c condChain) And(next Conditioner) Conditioner {
	c.next = next
	return c
}

func (c condChain) AndNot(next Conditioner) Conditioner {
	c.next = next
	c.not = true
	return c
}

func (c condChain) Or(next Conditioner) Conditioner {
	c.next = next
	c.or = true
	return c
}

func (c condChain) OrNot(next Conditioner) Conditioner {
	c.next = next
	c.or = true
	c.not = true
	return c
}

type Q map[string]Value

func (q Q) Predicate() map[string]Value {
	return q
}

func (q Q) Next() (Conditioner, bool, bool) {
	return nil, false, false
}

func (q Q) And(next Conditioner) Conditioner {
	return condChain{root: q, next: next}
}

func (q Q) AndNot(next Conditioner) Conditioner {
	return condChain{root: q, next: next, not: true}
}

func (q Q) Or(next Conditioner) Conditioner {
	return condChain{root: q, next: next, or: true}
}

func (q Q) OrNot(next Conditioner) Conditioner {
	return condChain{root: q, next: next, or: true, not: true}
}