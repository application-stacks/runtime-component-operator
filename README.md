# Application Runtime Operator
This generic Operator is capable of deploying any application image and can be imported into any runtime-specific Operator as library of application capabilities.  This architecture ensures compatibility and consistency between all runtime Operators, allowing everyone to benefit from the functionality added in this project.

![Architecture](docs/images/runtime_operators.png)

Currently the projects that are importing this library are:
- Appsody Operator: https://github.com/appsody/appsody-operator
- Open Liberty Operator: https://github.com/OpenLiberty/open-liberty-operator
