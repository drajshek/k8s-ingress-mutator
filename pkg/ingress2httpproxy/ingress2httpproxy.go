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
	supportedHosts   = "ingress-2-httpproxy/supported-hosts"
	unSupportedHosts = "ingress-2-httpproxy/unsupported-hosts"
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

//Mutate converts a Ingress into HTTPProxy
func (m *Mutator) Mutate() *MutatorOutput {
	return &MutatorOutput{
		HTTPProxy: m.buildHTTPProxy(),
	}

}

//Builds and returns the contour httpproxy
func (m *Mutator) buildHTTPProxy() contourv1.HTTPProxy {
	var httpproxyFqdn string
	// Meta data section start
	// Call the translateRoutes function to parse the rules section of ingress

	var httpAnnotations = make(map[string]string)
	hpTranslatedRoute := translateRoutes(m.input.Spec.Rules, m.log, &httpAnnotations)
	hp := contourv1.HTTPProxy{
		TypeMeta: v1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{

			Name:        m.input.ObjectMeta.Name,
			Annotations: httpAnnotations,
		},
		Spec: contourv1.HTTPProxySpec{

			Routes: hpTranslatedRoute,
		},
	}
	//Set up the wildcard DNS.
	log.Infof("%s", "%s", "Domain Received", m.domain)
	if m.domain != "" {
		normalizedDomain := m.domain
		// let's accept the domain starting with "*." or "."
		if m.domain[0:2] == "*." {
			normalizedDomain = m.domain[2:]
		} else if m.domain[0:1] == "." {
			normalizedDomain = m.domain[1:]
		}
		ocpRouteSplit := strings.SplitN(m.domain, ".", 2)
		httpproxyFqdn = ocpRouteSplit[0] + "." + normalizedDomain

	} else {
		// user did not specify the new wild card DNS
		httpproxyFqdn = m.input.Spec.Rules[0].Host
		log.Warnf("[%s] No new wildcard DNS domain specified. This mutation will use original domain from OCP route %s.", m.name, httpproxyFqdn)

	}

	//Assign the FQDN
	hp.Spec.VirtualHost = &contourv1.VirtualHost{
		Fqdn: httpproxyFqdn,
	}
	//Assign the secret name
	hp.Spec.VirtualHost.TLS = &contourv1.TLS{}
	hp.Spec.VirtualHost.TLS.SecretName = m.input.Spec.TLS[0].SecretName

	return hp

}

//Loop through the rules section and http paths
func translateRoutes(inrules []core.IngressRule, log logrus.FieldLogger, httpAnnotations *map[string]string) []contourv1.Route {
	var routes []contourv1.Route
	var unsupportedHosts string
	var annotations = make(map[string]string)
	var route = contourv1.Route{
		Conditions: []contourv1.MatchCondition{},
	}
	var routefinal = contourv1.Route{
		Conditions: []contourv1.MatchCondition{},
	}
	for i, inrule := range inrules {

		//var service1 = contourv1.Service{}

		if i >= 1 {

			unsupportedHosts += inrule.Host + ","
			log.Warnf("%s", "%s", "Unsupported host", inrule.Host)

		} else {

			annotations[supportedHosts] = inrule.Host
			log.Infof("%s", "%s", "Supported host", inrule.Host)
			for _, ipaths := range inrule.HTTP.Paths {

				//fmt.Println("looping")
				service, condition := translateService(ipaths.Backend, ipaths.Path)
				routefinal.Conditions = append(route.Conditions, condition)
				routefinal.Services = append(route.Services, service)
				routes = append(routes, routefinal)
			}
		}

	}
	annotations[unSupportedHosts] = strings.TrimRight(unsupportedHosts, ",")
	*httpAnnotations = annotations
	return routes
}

//create the route object and return to the translaterules function
func translateService(backend core.IngressBackend, prefix string) (contourv1.Service, contourv1.MatchCondition) {

	condition := contourv1.MatchCondition{}
	condition.Prefix = prefix

	service := contourv1.Service{
		Name: backend.ServiceName,
		Port: backend.ServicePort.IntValue(),
	}

	return service, condition
}
