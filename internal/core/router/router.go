package router

import (
	"net/http"
	"sync"
	"sync/atomic"
)

// HandleFunc is a function that can be registered to a route to handle HTTP
// requests. Like http.HandlerFunc, but has a third parameter for the values of
// wildcards (path variables).
type HandlerFunc func(http.ResponseWriter, *http.Request, Params)

// Param is a single URL parameter, consisting of a key and a value.
type Param struct {
	Key   string
	Value string
}

// Params is a Param-slice, as returned by the router.
// The slice is ordered, the first URL parameter is also the first slice value.
// It is therefore safe to read values by the index.
type Params []Param

// Get returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) Get(name string) string {
	for _, entry := range ps {
		if entry.Key == name {
			return entry.Value
		}
	}
	return ""
}

// ByName returns the value of the first Param which key matches the given name.
// If no matching Param is found, an empty string is returned.
func (ps Params) ByName(name string) string {
	return ps.Get(name)
}

// Router is a http.Handler which can be used to dispatch requests to different
// handler functions via configurable routes
type Router struct {
	trees atomic.Value // map[string]*node

	// Mutex for write operations (adding routes)
	mu sync.Mutex

	// Cached map for write operations
	writeTrees map[string]*node

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	PanicHandler func(http.ResponseWriter, *http.Request, interface{})

	// Configurable http.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	NotFound http.Handler

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// If it is not set, http.Error with http.StatusMethodNotAllowed is used.
	// The "Allow" header with allowed request methods is set.
	MethodNotAllowed http.Handler

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	// PanicHandler func(http.ResponseWriter, *http.Request, interface{})
}

// New returns a new initialized Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func New() *Router {
	r := &Router{
		writeTrees: make(map[string]*node),
	}
	r.trees.Store(make(map[string]*node))
	return r
}

// Handle registers a new request handle with the given path and method.
//
// For GET, POST, PUT, PATCH and DELETE requests the respective shortcut
// functions can be used.
//
// This function is intended for bulk loading and to allow the usage of less
// frequently used, non-standardized or custom methods (e.g. for internal
// communication with a proxy).
func (r *Router) Handle(method, path string, handle HandlerFunc) {
	if len(path) < 1 || path[0] != '/' {
		panic("path must begin with '/' in path '" + path + "'")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clone the trees for modification (Copy-On-Write)
	// Actually, we maintain a `writeTrees` map which is the source of truth for writes.
	// We then deep copy the affected tree and update the atomic value.
	// But wait, deep copying the whole tree on every insert is expensive if the tree is huge.
	// However, since we are building a microkernel, route updates (plugin loading) happen infrequently compared to reads.
	// And usually they happen in batches (server startup).
	// Let's implement a simpler approach:
	// The `writeTrees` is always the latest version.
	// On Handle, we update `writeTrees`.
	// Then we clone the *entire map* (shallow copy of map, but pointers to nodes are same).
	// BUT, `node` structure is not thread-safe for concurrent read/write if we modify it in place.
	// So `addRoute` modifies the node structure.
	// To be truly safe with atomic.Store, we should treat the old tree as immutable once stored.
	// So when we want to add a route, we must clone the *affected method's tree* entirely?
	// Or we can just use a RWMutex for the whole Router if we don't strictly need non-blocking reads?
	// The requirement says "O(1) ... atomic update".
	// Atomic update usually implies we swap the pointer.
	// To avoid race conditions where a reader is reading a node while a writer is modifying it (rebalancing children),
	// we must not modify nodes that are reachable by readers.
	// So we need to clone the tree for the specific method.

	root := r.writeTrees[method]
	if root == nil {
		root = &node{path: "/"}
		r.writeTrees[method] = root
	}

	// We need to clone the tree because readers might be traversing the current `root`.
	// However, implementing a full deep clone of the tree is tedious.
	// Alternative: Use a global RWMutex.
	// "High performance" usually means RLock for routing.
	// Is `atomic.Value` strictly better than RWMutex?
	// For routing (read-heavy), RWMutex is very fast, but `atomic.Value` is faster (no cache line bouncing on readers).
	// Given the constraints and typical Go patterns, let's try to do it right with COW.
	// Since `addRoute` modifies the tree structure in place, we MUST work on a copy.

	newRoot := deepCopyNode(root)
	newRoot.addRoute(path, handle)

	// Update the write cache with the new root
	r.writeTrees[method] = newRoot

	// Create a new map for the atomic value
	newTrees := make(map[string]*node)
	// Copy other methods from writeTrees
	for m, root := range r.writeTrees {
		newTrees[m] = root
	}

	// Store the new map
	r.trees.Store(newTrees)
}

func deepCopyNode(n *node) *node {
	if n == nil {
		return nil
	}
	cp := *n // shallow copy struct
	// deep copy slice
	if n.children != nil {
		cp.children = make([]*node, len(n.children))
		for i, child := range n.children {
			cp.children[i] = deepCopyNode(child)
		}
	}
	// deep copy wildcard
	if n.wildcard != nil {
		cp.wildcard = deepCopyNode(n.wildcard)
	}
	return &cp
}

// ServeHTTP makes the router implement the http.Handler interface.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	trees := r.trees.Load().(map[string]*node)
	root := trees[req.Method]

	if root != nil {
		if handle, params, _ := root.getValue(req.URL.Path); handle != nil {
			handle(w, req, params)
			return
		}
	}

	// Handle 404
	if r.NotFound != nil {
		r.NotFound.ServeHTTP(w, req)
	} else {
		http.NotFound(w, req)
	}
}

// Lookup allows the manual lookup of a method + path combo.
// This is e.g. useful to build a framework around this router.
func (r *Router) Lookup(method, path string) (HandlerFunc, Params, bool) {
	trees := r.trees.Load().(map[string]*node)
	root := trees[method]
	if root == nil {
		return nil, nil, false
	}
	handle, params, _ := root.getValue(path)
	return handle, params, handle != nil
}

// Helper Wrappers

func (r *Router) GET(path string, handle HandlerFunc) {
	r.Handle(http.MethodGet, path, handle)
}

func (r *Router) POST(path string, handle HandlerFunc) {
	r.Handle(http.MethodPost, path, handle)
}

func (r *Router) PUT(path string, handle HandlerFunc) {
	r.Handle(http.MethodPut, path, handle)
}

func (r *Router) DELETE(path string, handle HandlerFunc) {
	r.Handle(http.MethodDelete, path, handle)
}

func (r *Router) PATCH(path string, handle HandlerFunc) {
	r.Handle(http.MethodPatch, path, handle)
}

func (r *Router) HEAD(path string, handle HandlerFunc) {
	r.Handle(http.MethodHead, path, handle)
}

func (r *Router) OPTIONS(path string, handle HandlerFunc) {
	r.Handle(http.MethodOptions, path, handle)
}
