package ingress2httpproxy

import (
	"strings"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	unSupportedHosts = "unsupported-hosts"
)

// MutatorOutput contains the mutated output structures
type MutatorOutput struct {
	HTTPProxy contourv1.HTTPProxy
}

// Mutator contains common atttributes and the mutation input source structure
type Mutator struct {
	name   string
	log    logrus.FieldLogger
	input  core.Ingress
	domain string
}

// NewMutator creates a new Mutator. Clients of this API should set a meaningful name that can be used
// to easily identify the calling client.
func NewMutator(name string, log logrus.FieldLogger, ingress core.Ingress, domain string) Mutator {
	return Mutator{
		name:   name,
		log:    log,
		input:  ingress,
		domain: domain,
	}
}

// Mutate converts a Ingress into HTTPProxy
func (m *Mutator) Mutate() *MutatorOutput {
	return &MutatorOutput{
		HTTPProxy: m.buildHTTPProxy(),
	}
}

// buildHTTPProxy takes ingress object as an input and returns  Contour HTTPProxy
func (m *Mutator) buildHTTPProxy() contourv1.HTTPProxy {
	var httpProxyFqdn string
	var httpAnnotations = make(map[string]string)
	hpTranslatedRoute, httpAnnotations := m.createRoute(m.input.Spec.Rules, m.log, httpAnnotations)
	hp := contourv1.HTTPProxy{
		TypeMeta: v1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:        m.input.ObjectMeta.Name,
			Annotations: httpAnnotations,
			Namespace:   m.input.ObjectMeta.Namespace,
		},
		Spec: contourv1.HTTPProxySpec{
			Routes: hpTranslatedRoute,
		},
	}
	log.Warnf("[%s] No new wildcard DNS domain specified. This mutation will use original domain from Ingress Host %s.", m.name, httpProxyFqdn)
	httpProxyFqdn = m.input.Spec.Rules[0].Host
	if m.domain != "" {
		normalizedDomain := m.domain
		// let's accept the domain starting with "*." or "."
		if m.domain[0:2] == "*." {
			normalizedDomain = m.domain[2:]
		} else if m.domain[0:1] == "." {
			normalizedDomain = m.domain[1:]
		}
		ingressHostSplit := strings.SplitN(m.domain, ".", 2)
		httpProxyFqdn = ingressHostSplit[0] + "." + normalizedDomain
	}
	hp.Spec.VirtualHost = &contourv1.VirtualHost{
		Fqdn: httpProxyFqdn,
	}
	hp.Spec.VirtualHost.TLS = &contourv1.TLS{}
	hp.Spec.VirtualHost.TLS.SecretName = m.input.Spec.TLS[0].SecretName
	return hp
}

// createRoute creates the route object which includes condition and service details
func (m *Mutator) createRoute(inrules []core.IngressRule, log logrus.FieldLogger, httpAnnotations map[string]string) ([]contourv1.Route, map[string]string) {
	var routes []contourv1.Route
	var route = contourv1.Route{
		Conditions: []contourv1.MatchCondition{},
	}
	var routeFinal = contourv1.Route{
		Conditions: []contourv1.MatchCondition{},
	}
	log.Infof("%s", "%s", "Supported host", inrules[0].Host)
	for _, ipaths := range inrules[0].HTTP.Paths {
		routeFinal.Conditions = append(route.Conditions, contourv1.MatchCondition{Prefix: ipaths.Path})
		routeFinal.Services = append(route.Services, contourv1.Service{
			Name: ipaths.Backend.ServiceName,
			Port: ipaths.Backend.ServicePort.IntValue(),
		})
		routes = append(routes, routeFinal)
	}
	unsupportedHosts := make([]string, 0)
	for _, inrule := range inrules[1:] {
		unsupportedHosts = append(unsupportedHosts, inrule.Host)
	}
	log.Infof("%s", "%s", "Un Supported host", strings.Join(unsupportedHosts, ","))
	httpAnnotations[m.name+"/"+unSupportedHosts] = strings.Join(unsupportedHosts, ",")
	return routes, httpAnnotations
}
