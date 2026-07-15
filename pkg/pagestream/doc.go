// Package pagestream provides a small framework for Gomponents-rendered MPA
// pages that open one long-lived Datastar SSE transport.
//
// pagestream is intentionally opinionated: update streams carry Datastar signal
// patches only. Envelopes keep generation, coalescing, correlation, and trace
// metadata outside the browser's signal graph. Element morphs, route dispatch,
// authorization, and domain-specific patch generation belong outside this
// package. Command responses may use the package redirect helper so application
// code does not depend on Datastar directly.
package pagestream
