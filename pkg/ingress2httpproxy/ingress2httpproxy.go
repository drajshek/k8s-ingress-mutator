package ingress2httpproxy

import (
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"github.com/sirupsen/logrus"
	core "k8s.io/api/networking/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ingress = core.Ingress{}
var service = contourv1.Service{}
var route = contourv1.Route{
	Conditions: []contourv1.MatchCondition{},
}

var routefinal = contourv1.Route{
	Conditions: []contourv1.MatchCondition{},
}

//Mutate func recevies the plugin name, logger and ingress definition and returns the contour httpproxy
func Mutate(pluginName string, log logrus.FieldLogger, ingress core.Ingress) contourv1.HTTPProxy {

	// Meta data section start
	// Call the translateRoutes function to parse the rules section of ingress
	hpTranslatedRoute := translateRoutes(ingress.Spec.Rules)
	hp := contourv1.HTTPProxy{
		TypeMeta: v1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: v1.ObjectMeta{

			Name: ingress.ObjectMeta.Name,
		},
		Spec: contourv1.HTTPProxySpec{

			Routes: hpTranslatedRoute,
		},
	}
	//Assigning the fqdn
	httpproxyFqdn := ingress.Spec.Rules[0].Host

	//extract the Prefix of OCP route
	hp.Spec.VirtualHost = &contourv1.VirtualHost{
		Fqdn: httpproxyFqdn,
	}
	//Assign the secret name
	hp.Spec.VirtualHost.TLS = &contourv1.TLS{}
	hp.Spec.VirtualHost.TLS.SecretName = ingress.Spec.TLS[0].SecretName

	return hp

}

//Loop through the rules section and http paths
func translateRoutes(inrules []core.IngressRule) []contourv1.Route {
	var routes []contourv1.Route
	for _, inrule := range inrules {

		for i, ipaths := range inrule.HTTP.Paths {

			_ = translateService(ipaths.Backend, ipaths.Path, i)
			routes = append(routes, routefinal)
		}

	}
	return routes
}

// func translaterule(inrule core.IngressRule) contourv1.Route {

// 	//fmt.Println(inrule.HTTP.Paths[0].Backend.ServiceName)

// 	return route

// }

//create the route object and return to the translaterules function
func translateService(backend core.IngressBackend, prefix string, i int) contourv1.Service {

	condition := contourv1.MatchCondition{}
	condition.Prefix = prefix

	routefinal.Conditions = append(route.Conditions, condition)

	service := contourv1.Service{
		Name: backend.ServiceName,
		Port: backend.ServicePort.IntValue(),
	}
	routefinal.Services = append(route.Services, service)

	return service
}
