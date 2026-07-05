// Package pagestream provides a small framework for Gomponents-rendered MPA
// pages that open one long-lived Datastar SSE transport.
//
// pagestream is intentionally opinionated: streams carry Datastar signal
// patches only. Element morphs, script events, route dispatch, authorization,
// and domain-specific patch generation belong outside this package.
package pagestream
