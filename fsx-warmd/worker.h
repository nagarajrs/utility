#ifndef WORKER_H
#define WORKER_H

#include "queue.h"

void start_workers(work_queue_t *q, int per_node);
void wait_for_workers(void);

#endif
