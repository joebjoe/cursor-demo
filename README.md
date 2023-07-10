# cursor-demo

Playground for finding an intuitive pattern (both REST API + underlying db interface methods) for paginating results from a postgres database.


## Prerequisites

* Have Makefile installed (`brew install make`).
* Have Docker installed & running.
* [Import](https://docs.insomnia.rest/insomnia/import-export-data#import-data) Insomnia collection ([./insomnia.yaml]()).

## Make commands

* `seed` - for populating the db
* `server` - for starting the API server

## REST API

* `GET /users` for initiating a new search
* `GET /users/:cursor` for retrieving the next page

## Considerations

### Observability + Validation + Exception Handling
All quick and diry - could be iterated on.

### Why separate REST contracts?
The API could be implememented via a single route, and instead passing the cursor as a query param. However I value the intentionality gained from having separate routes. Knowing the intention of the caller simplifies the validatation/business logic and gives the ability to provide more intuitive error handling.


