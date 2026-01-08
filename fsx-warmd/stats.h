#ifndef STATS_H
#define STATS_H

#include <stdatomic.h>

extern atomic_ulong files_done;
extern atomic_ulong bytes_read;

void stats_init(void);
void stats_start_monitor(void);
void stats_stop_monitor(void);

#endif
