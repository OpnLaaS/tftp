package tftp

type (
	GetType uint8

	Options struct {
		ListenAddrTFTP, ListenAddrHTTP string
	}

	Server struct {
		Options     Options
		QuitChannel chan struct{}
	}

	Requestor struct {
		IPAddress  *string
		MacAddress *string
	}

	Context struct {
		getType  GetType
		filename string
		from     *Requestor
	}

	Getter interface {
		Get(getType GetType, ctx *Context) ([]byte, error)
	}
)
