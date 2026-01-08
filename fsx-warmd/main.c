#include <stdio.h>
#include <stdlib.h>

#include "queue.h"
#include "walker.h"
#include "worker.h"
#include "numa_util.h"
#include "stats.h"     // <-- NEW

#define QUEUE_SIZE 4096
#define WORKERS_PER_NODE 4

int main(int argc, char **argv) {
    if (argc != 2) {
        fprintf(stderr, "Usage: %s <fsx_mount>\n", argv[0]);
        return 1;
    }

    numa_init_or_die();

    work_queue_t queue;
    queue_init(&queue, QUEUE_SIZE);

    stats_init();               // <-- NEW
    stats_start_monitor();      // <-- NEW

    start_workers(&queue, WORKERS_PER_NODE);
    walk_files(argv[1], &queue);

    queue_close(&queue);
    wait_for_workers();

    stats_stop_monitor();       // <-- NEW

    queue_destroy(&queue);
    return 0;
}
