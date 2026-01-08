#define _GNU_SOURCE
#include <fcntl.h>
#include <unistd.h>
#include <stdlib.h>
#include <pthread.h>
#include <numa.h>

#include "worker.h"
#include "numa_util.h"
#include "stats.h"     // <-- NEW

#define BUF_SIZE (4 * 1024 * 1024)

static pthread_t *threads;
static int total_threads;

static void *worker_fn(void *arg) {
    work_queue_t *q = arg;

    pin_thread_to_numa();

    void *buf = numa_alloc_local(BUF_SIZE);

    while (1) {
        char *path = queue_pop(q);
        if (!path)
            break;

        int fd = open(path, O_RDONLY);
        if (fd >= 0) {
            ssize_t n;
            while ((n = read(fd, buf, BUF_SIZE)) > 0) {
                atomic_fetch_add(&bytes_read, n);   // <-- COUNT BYTES
            }
            close(fd);

            atomic_fetch_add(&files_done, 1);       // <-- COUNT FILE
        }

        free(path);
    }

    numa_free(buf, BUF_SIZE);
    return NULL;
}

void start_workers(work_queue_t *q, int per_node) {
    int nodes = numa_num_nodes();
    total_threads = nodes * per_node;

    threads = calloc(total_threads, sizeof(pthread_t));

    int idx = 0;
    for (int n = 0; n < nodes; n++) {
        for (int i = 0; i < per_node; i++) {
            pthread_create(&threads[idx++], NULL, worker_fn, q);
        }
    }
}

void wait_for_workers(void) {
    for (int i = 0; i < total_threads; i++) {
        pthread_join(threads[i], NULL);
    }
    free(threads);
}
