package ingress2httpproxy

import (
	"strings"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	supportedHosts   = "ingress-2-httpproxy/supported-hosts"
	unSupportedHosts = "ingress-2-httpproxy/unsupported-hosts"
)

//Mutate func recevies the plugin name, logger and ingress definition and returns the contour httpproxy
func Mutate(pluginName string, log logrus.FieldLogger, ingress core.Ingress, domain string) contourv1.HTTPProxy {
	var httpproxyFqdn string
	// Meta data section start
	// Call the translateRoutes function to parse the rules section of ingress

	var httpAnnotations = make(map[string]string)
	hpTranslatedRoute := translateRoutes(ingress.Spec.Rules, log, &httpAnnotations)
	hp := contourv1.HTTPProxy{
		TypeMeta: v1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{

			Name:        ingress.ObjectMeta.Name,
			Annotations: httpAnnotations,
		},
		Spec: contourv1.HTTPProxySpec{

			Routes: hpTranslatedRoute,
		},
	}
	//Set up the wildcard DNS.
	log.Infof("%s", "%s", "Domain Received", domain)
	if domain != "" {
		normalizedDomain := domain
		// let's accept the domain starting with "*." or "."
		if domain[0:2] == "*." {
			normalizedDomain = domain[2:]
		} else if domain[0:1] == "." {
			normalizedDomain = domain[1:]
		}
		ocpRouteSplit := strings.SplitN(domain, ".", 2)
		httpproxyFqdn = ocpRouteSplit[0] + "." + normalizedDomain

	} else {
		// user did not specify the new wild card DNS
		httpproxyFqdn = ingress.Spec.Rules[0].Host
		log.Warnf("[%s] No new wildcard DNS domain specified. This mutation will use original domain from OCP route %s.", pluginName, httpproxyFqdn)

	}

	//Assign the FQDN
	hp.Spec.VirtualHost = &contourv1.VirtualHost{
		Fqdn: httpproxyFqdn,
	}
	//Assign the secret name
	hp.Spec.VirtualHost.TLS = &contourv1.TLS{}
	hp.Spec.VirtualHost.TLS.SecretName = ingress.Spec.TLS[0].SecretName

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

// func translaterule(inrule core.IngressRule) contourv1.Route {

// 	//fmt.Println(inrule.HTTP.Paths[0].Backend.ServiceName)

// 	return route

// }

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
