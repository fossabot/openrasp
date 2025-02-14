/*
 * Copyright 2017-2019 Baidu Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.baidu.openrasp.cloud;

import java.util.concurrent.*;

/**
 * @description: 异步发送http请求
 * @author: anyang
 * @create: 2018/09/19 20:12
 */
public class ThreadPool {
    private final ExecutorService threadPool;

    public ThreadPool() {
        threadPool = Executors.newCachedThreadPool(new CustomThreadFactory());
    }

    public ExecutorService getThreadPool() {
        return threadPool;
    }

    class CustomThreadFactory implements ThreadFactory {
        @Override
        public Thread newThread(Runnable r) {
            Thread t = Executors.defaultThreadFactory().newThread(r);
            t.setDaemon(true);
            return t;
        }
    }
}