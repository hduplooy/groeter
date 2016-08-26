# groeter
## HTTP Server using hduplooy/groet

This is a basic HTTP server (using the build in ListenAndServe). A config file in JSON format is read from the argument line and parsed. From this file the entries are read with the type, what the matcher is and what the action is. For an entry a subrouter can be specified meaning that an entry is further subdivided with more matches.

The types are:
* path - the next element in the url path is match (eg. /path1/path2/...)
* exact - the entire path is matched exactly
* match - the current path element is matched by regulare expression
* domain - the domain of the request is matched to the match value
* host - the host part of the address is matched
* port - the port part of the url is matched
* protocol - either https or http is matched
* any - anything else is handled by this case

An example of a config file is:

	[{"type": "domain",
	  "match": "domain1.org",
	  "action": "fileserver",
	  "value": "/home/user/domain1.org"},
	 {"type": "domain",
	  "match": "domain2.com",
	  "action": "subrouter",
	  "router": [
		          {"type": "path",
		           "match": "app1",
		           "stripprefix": true,
		           "action": "reverseproxy",
		           "value": "http://localhost:8090/"},
		          {"type": "match",
		           "match": "app2*",
		           "action": "reverseproxy",
		           "value": "http://localhost:8100/"},
		          {"type": "any",
		           "action": "fileserver",
		           "value": "/home/user/domain2.com"}]}]

At root level if the call is made to domain domain1.org then a fileserver is provided using the path /home/user/domain1.org. If the domain is domain2.com then there is a subrouter with the following entries:
* With a path of app1 a reverseproxy is done to the localhost at port 8090. The prefix app1 is removed when doing the reverseproxy
* With a path of app2 and anything after it a reverseproxy is done to the localhost at port 8100
* Anything is is provided by a fileserver using path /home/user/domain2.com

