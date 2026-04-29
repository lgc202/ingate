package server

import discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"

type adsStreamState struct {
	requests map[string]*discoveryv3.DiscoveryRequest
	sent     map[string]adsSentResponse
}

type adsStreamRequest struct {
	request *discoveryv3.DiscoveryRequest
	err     error
}

type adsSentResponse struct {
	Version string
	Nonce   string
}

func newADSStreamState() adsStreamState {
	return adsStreamState{
		requests: make(map[string]*discoveryv3.DiscoveryRequest),
		sent:     make(map[string]adsSentResponse),
	}
}

func (s adsStreamState) isAcknowledged(request *discoveryv3.DiscoveryRequest, response *discoveryv3.DiscoveryResponse) bool {
	if request.GetResponseNonce() == "" || request.GetErrorDetail() != nil {
		return false
	}

	sent, ok := s.sent[response.GetTypeUrl()]
	return ok && sent.Version == response.GetVersionInfo() && sent.Nonce == request.GetResponseNonce()
}

func (s adsStreamState) hasSent(response *discoveryv3.DiscoveryResponse) bool {
	sent, ok := s.sent[response.GetTypeUrl()]
	return ok && sent.Version == response.GetVersionInfo() && sent.Nonce == response.GetNonce()
}

func (s adsStreamState) subscribedTypes() []string {
	orderedTypes := []string{clusterTypeURL, endpointTypeURL, routeTypeURL, listenerTypeURL}
	typeURLs := make([]string, 0, len(orderedTypes))
	for _, typeURL := range orderedTypes {
		if _, ok := s.requests[typeURL]; ok {
			typeURLs = append(typeURLs, typeURL)
		}
	}
	return typeURLs
}

func (s adsStreamState) recordRequest(request *discoveryv3.DiscoveryRequest) {
	if request.GetTypeUrl() == "" {
		return
	}
	s.requests[request.GetTypeUrl()] = request
}

func (s adsStreamState) record(response *discoveryv3.DiscoveryResponse) {
	s.sent[response.GetTypeUrl()] = adsSentResponse{
		Version: response.GetVersionInfo(),
		Nonce:   response.GetNonce(),
	}
}
