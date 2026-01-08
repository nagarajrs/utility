#include <stdlib.h>
#include <pthread.h>
#include "queue.h"

void queue_init(work_queue_t *q, int cap) {
    q->items = calloc(cap, sizeof(char *));
    q->cap = cap;
    q->head = q->tail = q->count = 0;
    q->closed = 0;
    pthread_mutex_init(&q->lock, NULL);
    pthread_cond_init(&q->not_empty, NULL);
    pthread_cond_init(&q->not_full, NULL);
}

void queue_push(work_queue_t *q, char *item) {
    pthread_mutex_lock(&q->lock);
    while (q->count == q->cap && !q->closed)
        pthread_cond_wait(&q->not_full, &q->lock);

    if (!q->closed) {
        q->items[q->tail] = item;
        q->tail = (q->tail + 1) % q->cap;
        q->count++;
        pthread_cond_signal(&q->not_empty);
    }
    pthread_mutex_unlock(&q->lock);
}

char *queue_pop(work_queue_t *q) {
    pthread_mutex_lock(&q->lock);
    while (q->count == 0 && !q->closed)
        pthread_cond_wait(&q->not_empty, &q->lock);

    char *item = NULL;
    if (q->count > 0) {
        item = q->items[q->head];
        q->head = (q->head + 1) % q->cap;
        q->count--;
        pthread_cond_signal(&q->not_full);
    }
    pthread_mutex_unlock(&q->lock);
    return item;
}

void queue_close(work_queue_t *q) {
    pthread_mutex_lock(&q->lock);
    q->closed = 1;
    pthread_cond_broadcast(&q->not_empty);
    pthread_cond_broadcast(&q->not_full);
    pthread_mutex_unlock(&q->lock);
}

void queue_destroy(work_queue_t *q) {
    free(q->items);
}
