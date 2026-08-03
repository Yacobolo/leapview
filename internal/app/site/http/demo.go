package http

import "github.com/Yacobolo/toolbelt/pagestream"

func visualShowcasePatch() pagestream.SignalPatch {
	return pagestream.SignalPatch{"visuals": visualDocumentation.Showcase}
}
