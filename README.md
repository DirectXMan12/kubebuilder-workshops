# Zero To Operator (KubeCon US 2019)

This contains the code for the KubeCon US 2019 workshop "Zero to
Operator", which walks through building the GuestBook operator, but this
time with newer Kubernetes features, like declarative defaulting and
server-side apply.

The corresponding slides can be found at
https://pres.metamagical.dev/kubecon-us-2019.

Other workshops can be found on other branches.  Check out the [master
branch's
README](https://github.com/DirectXMan12/kubebuilder-workshops/blob/master/README.md)
for more info.

# Starting Out

Each commit contains progressively more details on building the workshop.
Take a look at the commit messages, and then start from the
`start-kubecon-us-2019` tag with

```bash
$ git clone https://github.com/directxman12/kubebuilder-workshops --branch start-kubecon-us-2019
```

or (from within the repository)

```bash
$ git checkout start-kubecon-us-2019
```

You can also start at HEAD, but that spoils the fun :-).

# Contributing

If you'd like to contribute fixes, check out
[CONTRIBUTING.md](CONTRIBUTING.md).
