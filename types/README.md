# package types

This package is used by other packages in this repository. It contains the generic types that are used by the other packages.
Specific types are defined in the packages that use them.

This package must not import any other package from this repository in order to avoid circular dependencies. If you need to import another package, you should move the type to that package.