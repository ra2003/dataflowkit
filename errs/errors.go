package errs

//400 Bad Request
//
//The server cannot or will not process the request due to an apparent client error (e.g., malformed request syntax, size too large, invalid request message framing, or deceptive request routing).
type BadRequest struct {
	Err error
}
func (e *BadRequest) Error() string { return e.Err.Error() }

//403 Forbidden
//
//Client does not have access rights to the content caused by robots.txt restrictions.
type ForbiddenByRobots struct {
	URL string
}
func (e *ForbiddenByRobots) Error() string { return e.URL + ": forbidden by robots.txt" }

//403 Forbidden
//
//Client does not have access rights to the content so server is rejecting to give proper response.
type Forbidden struct {
	URL string
}
func (e *Forbidden) Error() string { return e.URL + ": forbidden" }


//404 Not Found
//
//Server can not find requested resource. This response code probably is most famous one due to its frequency to occur in web.
type NotFound struct {
	URL    string
}
func (e *NotFound) Error() string {
	return e.URL + ": Not found" 	
}

//502 Bad Gateway
//
//This error response means that the server, while working as a gateway to get a response needed to handle the request, got an invalid response.
type BadGateway struct {	
}
func (e *BadGateway) Error() string {
	return "Invalid response from server" 	
}

//504 Gateway Time-out
//
//This error response is given when the server is acting as a gateway and cannot get a response in time.
type GatewayTimeout struct {
}
func (e *GatewayTimeout) Error() string {
	return "Timeout exceeded rendering page" 	
}


//All the rest. Unspecified errors 
type Error struct {
	Err string
}
func (e *Error) Error() string {
	return e.Err
}