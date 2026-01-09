window.BENCHMARK_DATA = {
  "lastUpdate": 1767970407851,
  "repoUrl": "https://github.com/PoyrazK/thecloud",
  "entries": {
    "Go Benchmarks": [
      {
        "commit": {
          "author": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "committer": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "distinct": true,
          "id": "66677b5d976641a5e9eb591059f9944016e4a515",
          "message": "ci: add write permissions to benchmarks workflow",
          "timestamp": "2026-01-09T15:12:58+03:00",
          "tree_id": "fae73ef4a542441e13f73b3fe8b43bbb8a95e077",
          "url": "https://github.com/PoyrazK/thecloud/commit/66677b5d976641a5e9eb591059f9944016e4a515"
        },
        "date": 1767960802284,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkInstanceService_List",
            "value": 1.895,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "639451438 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceService_List - ns/op",
            "value": 1.895,
            "unit": "ns/op",
            "extra": "639451438 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceService_List - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "639451438 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceService_List - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "639451438 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCService_Get",
            "value": 138.7,
            "unit": "ns/op\t     192 B/op\t       2 allocs/op",
            "extra": "9101067 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCService_Get - ns/op",
            "value": 138.7,
            "unit": "ns/op",
            "extra": "9101067 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCService_Get - B/op",
            "value": 192,
            "unit": "B/op",
            "extra": "9101067 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCService_Get - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "9101067 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "committer": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "distinct": true,
          "id": "0bbf8279130caa7edf93beeb40843983c9a902fe",
          "message": "Merge branch 'fix/lint-issues': Major code quality improvements\n\n- Reduced cognitive complexity in InstanceService and LibvirtAdapter\n- Introduced parameter structs to reduce function parameter counts\n- Eliminated duplicate string literals across codebase\n- Improved security by addressing potential secret exposure in tests\n- All tests passing with no regressions",
          "timestamp": "2026-01-09T17:49:49+03:00",
          "tree_id": "26b7a92c928743ffa90c52dfabf4f1c14e7cb185",
          "url": "https://github.com/PoyrazK/thecloud/commit/0bbf8279130caa7edf93beeb40843983c9a902fe"
        },
        "date": 1767970227717,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkInstanceServiceList",
            "value": 1.449,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "828889226 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - ns/op",
            "value": 1.449,
            "unit": "ns/op",
            "extra": "828889226 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "828889226 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "828889226 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet",
            "value": 126.1,
            "unit": "ns/op\t     192 B/op\t       2 allocs/op",
            "extra": "9403125 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - ns/op",
            "value": 126.1,
            "unit": "ns/op",
            "extra": "9403125 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - B/op",
            "value": 192,
            "unit": "B/op",
            "extra": "9403125 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "9403125 times\n4 procs"
          }
        ]
      },
      {
        "commit": {
          "author": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "committer": {
            "email": "83272398+PoyrazK@users.noreply.github.com",
            "name": "Poyraz Küçükarslan",
            "username": "PoyrazK"
          },
          "distinct": true,
          "id": "ec4b81f55c22a27479bab40c5c666b8775badb93",
          "message": "fix(ci): start API server before running load tests\n\n- Add PostgreSQL service container for database\n- Build and start API server in background\n- Wait for health check to pass before running k6\n- Properly cleanup server process after tests\n- Fixes connection refused errors in load test workflow",
          "timestamp": "2026-01-09T17:52:58+03:00",
          "tree_id": "9ae91f74a73649766d838f3be980d9bb58f88f00",
          "url": "https://github.com/PoyrazK/thecloud/commit/ec4b81f55c22a27479bab40c5c666b8775badb93"
        },
        "date": 1767970399186,
        "tool": "go",
        "benches": [
          {
            "name": "BenchmarkInstanceServiceList",
            "value": 1.867,
            "unit": "ns/op\t       0 B/op\t       0 allocs/op",
            "extra": "642284600 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - ns/op",
            "value": 1.867,
            "unit": "ns/op",
            "extra": "642284600 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - B/op",
            "value": 0,
            "unit": "B/op",
            "extra": "642284600 times\n4 procs"
          },
          {
            "name": "BenchmarkInstanceServiceList - allocs/op",
            "value": 0,
            "unit": "allocs/op",
            "extra": "642284600 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet",
            "value": 135.6,
            "unit": "ns/op\t     192 B/op\t       2 allocs/op",
            "extra": "8985362 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - ns/op",
            "value": 135.6,
            "unit": "ns/op",
            "extra": "8985362 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - B/op",
            "value": 192,
            "unit": "B/op",
            "extra": "8985362 times\n4 procs"
          },
          {
            "name": "BenchmarkVPCServiceGet - allocs/op",
            "value": 2,
            "unit": "allocs/op",
            "extra": "8985362 times\n4 procs"
          }
        ]
      }
    ]
  }
}