#include <pthread.h>
#include <stdio.h>

#define ITERATION_COUNT 1000000

int i = 0;
pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;

void* incrementingThreadFunction(){
    for (int j = 0; j < ITERATION_COUNT; j++) {
        pthread_mutex_lock(&mutex);
        i++;
        pthread_mutex_unlock(&mutex);
    }
    return NULL;
}

void* decrementingThreadFunction(){
    for (int j = 0; j < ITERATION_COUNT; j++) {
        pthread_mutex_lock(&mutex);
        i--;
        pthread_mutex_unlock(&mutex);
    }
    return NULL;
}


int main(){
    pthread_t incrementThread;
    pthread_t decrementThread;
    
    pthread_create(&incrementThread, NULL, incrementingThreadFunction, NULL);
    pthread_create(&decrementThread, NULL, decrementingThreadFunction, NULL);
    
    pthread_join(incrementThread, NULL);
    pthread_join(decrementThread, NULL);
    
    printf("The magic number is: %d\n", i);
    return 0;
}
