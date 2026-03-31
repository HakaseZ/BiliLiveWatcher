package notifier

type Notifier struct {
	Type string
}

type Bark struct {
	Notifier
	urls []string
}
