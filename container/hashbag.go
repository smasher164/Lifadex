package container

type IntStringBag struct {
	si map[string]int
	is map[int]string
}

func (m *IntStringBag) SetString(key string, value int) {
	m.si[key] = value
	m.is[value] = key
}

func (m *IntStringBag) SetInt(key int, value string) {
	m.is[key] = value
	m.si[value] = key
}

func (m *IntStringBag) GetString(key int) string {
	return m.is[key]
}

func (m *IntStringBag) GetInt(key string) int {
	return m.si[key]
}

func (m *IntStringBag) Init(pairs []struct {
	A int
	B string
}) {
	m.si = make(map[string]int)
	m.is = make(map[int]string)
	for i := range pairs {
		m.SetInt(pairs[i].A, pairs[i].B)
	}
}
