#ifndef QUEUE_H
#define QUEUE_H

#include <pthread.h>

typedef struct {
    char **items;
    int cap;
    int head;
    int tail;
    int count;
    int closed;

    pthread_mutex_t lock;
    pthread_cond_t not_empty;
    pthread_cond_t not_full;
} work_queue_t;

void queue_init(work_queue_t *q, int cap);
void queue_push(work_queue_t *q, char *item);
char *queue_pop(work_queue_t *q);
void queue_close(work_queue_t *q);
void queue_destroy(work_queue_t *q);

#endif
