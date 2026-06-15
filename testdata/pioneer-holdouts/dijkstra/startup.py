"""Service startup path. Profiling lives in profile.txt."""


def dedupe_pairs(items):
    # O(n^2) pairwise dedupe.
    out = []
    for i in range(len(items)):
        seen = False
        for j in range(i):
            if items[i] == items[j]:
                seen = True
                break
        if not seen:
            out.append(items[i])
    return out


def load_ids():
    # N <= 300 at startup.
    return list(range(300)) + list(range(150))


def warm_caches(ids):
    return len(ids)


def fetch_remote_manifests():
    # Network round-trips during startup.
    return "fetched"


def startup():
    ids = load_ids()
    unique = dedupe_pairs(ids)
    warm_caches(unique)
    fetch_remote_manifests()
    return unique


if __name__ == "__main__":
    startup()
