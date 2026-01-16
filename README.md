#GO Load Balancer.
A Load balancer that implements a round robin algorithm on healthy server backend APIs. While not intended to be industry standard, it acts as a proof-of-concept for learning purposes.

Oftentimes when a server backend receives too much of a load, it can be important to make sure it is balanced so that it doesn't become an issue. This load balancer is meant to act as a basic version of more common, industry-standard loadbalancers like what is found in AWS.

#Features
- Connection to localhost test backends (also included in program)
- Backend cycling
- Round Robin Algorithm Implementation
- Healthy / Unhealthy consistent backend checking

##Requirements
- Go: 1.25.6
- Windows Operating System

## License 
MIT
