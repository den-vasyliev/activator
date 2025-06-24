package activator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	routePattern = regexp.MustCompile(`^/preview/(?P<env>[a-z0-9-]+)/(?P<svc>[a-z0-9-]+)(?P<rest>/.*)?$`)
	client       *kubernetes.Clientset
	namespace    = "preview"
	cache        = newReadinessCache(5 * time.Minute)
)

func Start(kubeconfigPath string) {
	var cfg *rest.Config
	var err error
	if kubeconfigPath == "" {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			log.Error().Err(err).Msg("Error building in-cluster kubeconfig")
			return
		}
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			log.Error().Err(err).Msgf("Error loading kubeconfig from %s", kubeconfigPath)
			return
		}
	}
	client, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error().Err(err).Msg("Error creating k8s client")
		return
	}

	http.HandleFunc("/preview/", activatorHandler)
	log.Info().Msg("Activator listening on :8080")
	log.Fatal().Err(http.ListenAndServe(":8080", nil))
}

func activatorHandler(w http.ResponseWriter, r *http.Request) {
	match := routePattern.FindStringSubmatch(r.URL.Path)
	if match == nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	env := match[1]
	svc := match[2]
	rest := match[3]
	if rest == "" {
		rest = "/"
	}

	serviceName := fmt.Sprintf("%s-ecosystem-%s", env, svc)
	fqdn := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	if !cache.IsReady(serviceName) {
		if !isDeploymentReady(namespace, serviceName) {
			log.Info().Msgf("Deployment %s not ready. Scaling up...", serviceName)
			err := scaleDeployment(namespace, serviceName, 1)
			if err != nil {
				http.Error(w, fmt.Sprintf("Scale up failed: %v", err), http.StatusInternalServerError)
				return
			}
			waitForDeployment(namespace, serviceName, 120*time.Second)
		}
		cache.MarkReady(serviceName)
	}

	target, _ := url.Parse(fmt.Sprintf("http://%s", fqdn))
	rp := httputil.NewSingleHostReverseProxy(target)
	r.URL.Path = rest
	r.Host = target.Host

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Msg("Reverse proxy error")
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
	}

	rp.ServeHTTP(w, r)
}

func isDeploymentReady(ns, name string) bool {
	deploy, err := client.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil || deploy.Status.ReadyReplicas < 1 {
		return false
	}
	return true
}

func scaleDeployment(ns, name string, replicas int32) error {
	scale, err := client.AppsV1().Deployments(ns).GetScale(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	scale.Spec.Replicas = replicas
	_, err = client.AppsV1().Deployments(ns).UpdateScale(context.TODO(), name, scale, metav1.UpdateOptions{})
	return err
}

func waitForDeployment(ns, name string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			log.Info().Msgf("Timeout waiting for %s to be ready", name)
			return
		default:
			if isDeploymentReady(ns, name) {
				log.Info().Msgf("Deployment %s is ready", name)
				return
			}
			time.Sleep(3 * time.Second)
		}
	}
}

// --- Simple in-memory readiness cache ---

type readinessCache struct {
	mu     sync.Mutex
	cache  map[string]time.Time
	expiry time.Duration
}

func newReadinessCache(expiry time.Duration) *readinessCache {
	return &readinessCache{
		cache:  make(map[string]time.Time),
		expiry: expiry,
	}
}

func (c *readinessCache) IsReady(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.cache[name]
	return ok && time.Since(t) < c.expiry
}

func (c *readinessCache) MarkReady(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[name] = time.Now()
}
