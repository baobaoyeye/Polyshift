package router

type nodeType uint8

const (
	static nodeType = iota // default
	root
	param
	catchAll
)

type node struct {
	path     string
	indices  string
	nType    nodeType
	priority uint32
	children []*node
	wildcard *node // Points to a param or catchAll node
	handle   HandlerFunc
}

// increments priority of the given child and reorders if necessary
func (n *node) incrementChildPrio(pos int) int {
	cs := n.children
	cs[pos].priority++
	prio := cs[pos].priority

	// Adjust position (move to front)
	newPos := pos
	for ; newPos > 0 && cs[newPos-1].priority < prio; newPos-- {
		// Swap node positions
		cs[newPos-1], cs[newPos] = cs[newPos], cs[newPos-1]
	}

	// Build new index string
	if newPos != pos {
		n.indices = n.indices[:newPos] + // Unchanged prefix
			n.indices[pos:pos+1] + // Moved char
			n.indices[newPos:pos] + // Shifted chars
			n.indices[pos+1:] // Remaining suffix
	}

	return newPos
}

func (n *node) split(i int) {
	child := node{
		path:     n.path[i:],
		nType:    static,
		indices:  n.indices,
		children: n.children,
		wildcard: n.wildcard,
		handle:   n.handle,
		priority: n.priority - 1,
	}

	n.children = []*node{&child}
	n.indices = string([]byte{n.path[i]})
	n.path = n.path[:i]
	n.handle = nil
	n.wildcard = nil
}

// addRoute adds a node with the given handle to the path.
func (n *node) addRoute(path string, handle HandlerFunc) {
	fullPath := path
	n.priority++

	// Empty tree
	if len(n.path) == 0 && len(n.children) == 0 && n.wildcard == nil {
		n.insertChild(path, fullPath, handle)
		n.nType = root
		return
	}

walk:
	for {
		// Find the longest common prefix.
		i := longestCommonPrefix(path, n.path)

		// Split edge
		if i < len(n.path) {
			n.split(i)
		}

		// Make new node a child of this node
		if i < len(path) {
			path = path[i:]
			c := path[0]

			// Check if a child with the next path byte exists
			for i, max := 0, len(n.indices); i < max; i++ {
				if c == n.indices[i] {
					i = n.incrementChildPrio(i)
					n = n.children[i]
					continue walk
				}
			}

			// If path starts with wildcard identifier
			if c == ':' || c == '*' {
				if n.wildcard == nil {
					if c == '*' && len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
						n.split(len(n.path) - 1)
						n.wildcard = &node{
							nType:    catchAll,
							path:     "/" + path,
							handle:   handle,
							priority: 1,
						}
						return
					}

					n.insertChild(path, fullPath, handle)
					return
				}
				n = n.wildcard
				// Check wildcard conflict
				wildName, _, _ := findWildcard(path)
				if n.path != wildName {
					panic("wildcard route '" + wildName +
						"' conflicts with existing wildcard '" + n.path +
						"' in path '" + fullPath + "'")
				}

				if len(path) > len(wildName) {
					path = path[len(wildName):]
					if n.nType == param {
						continue walk
					}
					// catchAll must be the end
					panic("catchAll conflicts")
				}

				if n.handle != nil {
					panic("handler already registered")
				}
				n.handle = handle
				return
			}

			// Otherwise insert it as static child
			n.indices += string([]byte{c})
			child := &node{}
			n.children = append(n.children, child)
			n.incrementChildPrio(len(n.indices) - 1)
			n = child
			n.insertChild(path, fullPath, handle)
			return
		}

		// Otherwise add handle to current node
		if n.handle != nil {
			panic("a handle is already registered for path '" + fullPath + "'")
		}
		n.handle = handle
		return
	}
}

func (n *node) insertChild(path, fullPath string, handle HandlerFunc) {
	for {
		// Find prefix until first wildcard
		wildcard, i, valid := findWildcard(path)
		if i < 0 { // No wildcard found
			break
		}

		if !valid {
			panic("only one wildcard per path segment is allowed, has: '" +
				wildcard + "' in path '" + fullPath + "'")
		}

		if i > 0 {
			n.path = path[:i]
			path = path[i:]
		}

		// param
		if wildcard[0] == ':' {
			n.wildcard = &node{
				nType: param,
				path:  wildcard,
			}
			n = n.wildcard
			n.priority++

			if len(wildcard) < len(path) {
				path = path[len(wildcard):]
				child := &node{
					priority: 1,
				}
				n.children = []*node{child}
				n = child
				continue
			}

			n.handle = handle
			return
		}

		// catchAll
		if len(wildcard) != len(path) {
			panic("catch-all routes are only allowed at the end of the path in path '" + fullPath + "'")
		}

		if len(n.path) > 0 && n.path[len(n.path)-1] == '/' {
			n.path = n.path[:len(n.path)-1]
			path = "/" + path
			n.wildcard = &node{
				nType:    catchAll,
				path:     path,
				handle:   handle,
				priority: 1,
			}
			return
		}

		if i > 0 {
			// debug
			// println("i:", i, "n.path:", n.path, "path:", path)
			if n.path[len(n.path)-1] != '/' {
				panic("no / before catch-all in path '" + fullPath + "'")
			}
		}

		n.wildcard = &node{
			nType:    catchAll,
			path:     path,
			handle:   handle,
			priority: 1,
		}

		return
	}

	n.path = path
	n.handle = handle
}

func (n *node) getValue(path string) (handle HandlerFunc, params Params, tsr bool) {
walk:
	for {
		prefix := n.path
		if len(path) > len(prefix) {
			if path[:len(prefix)] == prefix {
				path = path[len(prefix):]

				// Try static children first
				c := path[0]
				indices := n.indices
				for i, max := 0, len(indices); i < max; i++ {
					if c == indices[i] {
						n = n.children[i]
						continue walk
					}
				}

				// If no static match, try wildcard
				if n.wildcard != nil {
					n = n.wildcard
					switch n.nType {
					case param:
						end := 0
						for end < len(path) && path[end] != '/' {
							end++
						}

						if params == nil {
							params = make(Params, 0, 1)
						}
						if cap(params) == len(params) {
							newParams := make(Params, len(params), len(params)+1)
							copy(newParams, params)
							params = newParams
						}
						params = append(params, Param{
							Key:   n.path[1:],
							Value: path[:end],
						})

						if end < len(path) {
							if len(n.children) > 0 {
								path = path[end:]
								n = n.children[0]
								continue walk
							}
							return
						}

						if handle = n.handle; handle != nil {
							return
						}
						// No TSR for now for simplicity
						return

					case catchAll:
						if params == nil {
							params = make(Params, 0, 1)
						}
						if cap(params) == len(params) {
							newParams := make(Params, len(params), len(params)+1)
							copy(newParams, params)
							params = newParams
						}

						key := n.path
						if len(key) > 0 && key[0] == '/' {
							key = key[1:]
						}
						if len(key) > 0 && key[0] == '*' {
							key = key[1:]
						}

						params = append(params, Param{
							Key:   key,
							Value: path,
						})
						handle = n.handle
						return
					}
				}

				// TSR
				tsr = (path == "/" && n.handle != nil)
				return
			}
		} else if path == prefix {
			if handle = n.handle; handle != nil {
				return
			}
			if path == "/" && n.wildcard != nil && n.nType != root {
				tsr = true
				return
			}
			return
		}
		return
	}
}

func findWildcard(path string) (wildcard string, i int, valid bool) {
	for start, c := range []byte(path) {
		if c != ':' && c != '*' {
			continue
		}
		valid = true
		for end, c := range []byte(path[start+1:]) {
			switch c {
			case '/':
				return path[start : start+1+end], start, valid
			case ':', '*':
				valid = false
			}
		}
		return path[start:], start, valid
	}
	return "", -1, false
}

func longestCommonPrefix(a, b string) int {
	i := 0
	max := min(len(a), len(b))
	for i < max && a[i] == b[i] {
		i++
	}
	return i
}

func min(a, b int) int {
	if a <= b {
		return a
	}
	return b
}
