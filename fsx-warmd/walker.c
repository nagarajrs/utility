#define _XOPEN_SOURCE 700
#include <ftw.h>
#include <string.h>
#include "walker.h"

static work_queue_t *g_queue;

static int visit(const char *path, const struct stat *sb,
                 int typeflag, struct FTW *ftwbuf) {
    if (typeflag == FTW_F) {
        queue_push(g_queue, strdup(path));
    }
    return 0;
}

void walk_files(const char *root, work_queue_t *queue) {
    g_queue = queue;
    nftw(root, visit, 32, FTW_PHYS);
}
