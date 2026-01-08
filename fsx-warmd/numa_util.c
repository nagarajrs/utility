#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <numa.h>
#include <sched.h>
#include "numa_util.h"

void numa_init_or_die(void) {
    if (numa_available() < 0) {
        fprintf(stderr, "NUMA not available\n");
        exit(1);
    }
}

int numa_num_nodes(void) {
    return numa_num_configured_nodes();
}

void pin_thread_to_numa(void) {
    int node = sched_getcpu() % numa_num_configured_nodes();
    struct bitmask *cpus = numa_allocate_cpumask();
    numa_node_to_cpus(node, cpus);
    numa_sched_setaffinity(0, cpus);
    numa_free_cpumask(cpus);
}
