<!--- master only -->
> ![warning](../../images/warning.png) This document applies to the HEAD of the calico-docker source tree.
>
> View the calico-docker documentation for the latest release [here](https://github.com/projectcalico/calico-containers/blob/v0.20.0/README.md).
<!--- else
> You are viewing the calico-docker documentation for release **release**.
<!--- end of master only -->

# Troubleshooting
This article contains Kubernetes specific troubleshooting advice for Calico.  See also the [main Calico troubleshooting](../../Troubleshooting.md) guide.

## Viewing Logs
The Calico CNI plugin emits logs to the `/var/log/calico/cni/` directory.  All CNI plugin logs for a node will be emitted to
that directory.  Searching for logs with "ERROR" or "WARN" level is a good place to start if you are having trouble. 

The log level can be configured via the CNI network configuration file, by changing the value of the `log_level` and `log_level_stderr` keys.
By default, the plugin will only emit "info" level and higher.  Valid log levels are `debug`, `info`, `warn`, and
`error`.

When logging to stderr, the Calico CNI logs can be found in the kubelet logs for a given node.  For deployments using `systemd`,
you can do this via `journalctl`.


[![Analytics](https://calico-ga-beacon.appspot.com/UA-52125893-3/calico-containers/docs/cni/kubernetes/Troubleshooting.md?pixel)](https://github.com/igrigorik/ga-beacon)
