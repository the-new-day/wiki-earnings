package articleinfo

type Section struct {
	Level int
	Line  string
}

type Info struct {
	Title      string
	Words      []string
	Categories []string
	Links      []string
	Images     []string
	Sections   []Section
	Templates  []string
}
