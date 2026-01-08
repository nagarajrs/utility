#ifndef NUMA_UTIL_H
#define NUMA_UTIL_H

void numa_init_or_die(void);
int numa_num_nodes(void);
void pin_thread_to_numa(void);

#endif
