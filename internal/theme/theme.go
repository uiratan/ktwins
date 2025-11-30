package theme

const (
	Title  = "[orange::b]"
	Header = "[skyblue::b]"
	Red    = "[red]"
	Green  = "[green]"
	Reset  = "[-:-:-]"
)

func ColorFor(active bool) string {
	if active {
		return Header
	}
	return Title
}
