package app

import (
	"fmt"
	"sync"

	documentcontent "github.com/Yacobolo/leapview/docs"
	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	docsearch "github.com/Yacobolo/leapview/internal/app/site/search/sqlite"
	"github.com/Yacobolo/leapview/internal/platform/http/cursorsigning"
)

var embeddedAgentDocumentation struct {
	once          sync.Once
	documentation agentmodule.Documentation
	err           error
}

func buildAgentDocumentation() (agentmodule.Documentation, error) {
	embeddedAgentDocumentation.once.Do(func() {
		index, err := docsearch.Open(documentcontent.Files, docsearch.Filename)
		if err != nil {
			embeddedAgentDocumentation.err = fmt.Errorf("open embedded documentation search index: %w", err)
			return
		}
		embeddedAgentDocumentation.documentation, embeddedAgentDocumentation.err = agentmodule.BuildDocumentation(
			documentcontent.Files,
			index,
			cursorsigning.Sign,
			cursorsigning.Verify,
		)
		if embeddedAgentDocumentation.err != nil {
			_ = index.Close()
			embeddedAgentDocumentation.err = fmt.Errorf("build embedded agent documentation: %w", embeddedAgentDocumentation.err)
		}
	})
	return embeddedAgentDocumentation.documentation, embeddedAgentDocumentation.err
}
