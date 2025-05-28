## Architecture:
- Using Domain Driven Design(DDD).
	- adapters (http_handlers, db_repositories, db_models etc) --(depends on)--> services (service, dto) -> domain (entities, repositories, validations etc)

- A loosely coupled monolith, with separate entry points for the server and workers.
	- #### Worker: 
		- Loads the compressed coupon code files, processes and generates a single valid coupons list file. Then triggers a http call to server, so the server can load coupons list to cache.
		- This process runs both on server startup and also on-demand, when the input files are updated.
		- Will be deployed as a separate docker container. A long running worker that continuosly polls to listen for file changes. (If files are not updated, this could be a single removable job).
- A UI server for scalar api client.

