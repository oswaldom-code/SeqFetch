package trivy

default ignore = false

ignore {
    ignored_packages := {
        "stdlib"
    }
    ignored_packages[input.PkgName]
}

ignore {
    ignored_cves := {
    }
    ignored_cves[input.VulnerabilityID]
}
