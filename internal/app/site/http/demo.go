package http

import "github.com/flidai/leapview/pkg/pagestream"

func visualShowcasePatch() pagestream.SignalPatch {
	return pagestream.SignalPatch{"visuals": visualDocumentation.Showcase}
}
