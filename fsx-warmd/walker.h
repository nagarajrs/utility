#ifndef WALKER_H
#define WALKER_H

#include <sys/stat.h>
#include "queue.h"

void walk_files(const char *root, work_queue_t *queue);

#endif
