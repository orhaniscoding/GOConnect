package systray

type MenuItem struct {
    Title     string
    Tooltip   string
    ClickedCh chan struct{}
}

func (m *MenuItem) SetTitle(t string) { m.Title = t }

var items []*MenuItem

func Run(onReady func(), onExit func()) {
    if onReady != nil { onReady() }
    if onExit != nil { onExit() }
}

func Quit() {}

func AddMenuItem(title, tooltip string) *MenuItem {
    mi := &MenuItem{Title: title, Tooltip: tooltip, ClickedCh: make(chan struct{}, 1)}
    items = append(items, mi)
    return mi
}

