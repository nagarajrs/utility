#include <stdio.h>
#include <unistd.h>
#include <time.h>
#include <pthread.h>
#include "stats.h"

atomic_ulong files_done = 0;
atomic_ulong bytes_read = 0;

static pthread_t monitor;
static int running = 1;

static void *monitor_fn(void *arg) {
    time_t start = time(NULL);

    while (running) {
        sleep(2);

        unsigned long f = atomic_load(&files_done);
        unsigned long b = atomic_load(&bytes_read);

        double elapsed = difftime(time(NULL), start);
        double mb = b / (1024.0 * 1024.0);
        double rate = elapsed > 0 ? mb / elapsed : 0;

        fprintf(stderr,
            "[warmup] files=%lu  data=%.1f MB  rate=%.1f MB/s  elapsed=%.0fs\n",
            f, mb, rate, elapsed);
    }
    return NULL;
}

void stats_init(void) {
    atomic_store(&files_done, 0);
    atomic_store(&bytes_read, 0);
}

void stats_start_monitor(void) {
    pthread_create(&monitor, NULL, monitor_fn, NULL);
}

void stats_stop_monitor(void) {
    running = 0;
    pthread_join(monitor, NULL);
}
