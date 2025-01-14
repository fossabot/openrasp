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

package com.baidu.openrasp.hook.server;

import com.baidu.openrasp.HookHandler;
import com.baidu.openrasp.hook.AbstractClassHook;
import com.baidu.openrasp.tool.Reflection;

/**
 * Created by tyy on 18-2-8.
 *
 * 服务器请求 hook 点基类
 */
public abstract class ServerRequestHook extends AbstractClassHook {

    public ServerRequestHook() {
        isNecessary = true;
    }

    /**
     * (none-javadoc)
     * tyy
     *
     * @see com.baidu.openrasp.hook.AbstractClassHook#getType()
     */
    @Override
    public String getType() {
        return "request";
    }

    /**
     * catalina 请求 hook 点检测入口
     *
     * @param filter   ApplicationFilterChain 实例本身
     * @param request  请求实体
     * @param response 响应实体
     */
    public static void checkRequest(Object filter, Object request, Object response) {
        HookHandler.checkFilterRequest(filter, request, response);
    }

    public static void checkRequest(Object undertow, Object http) {
        try {
            ClassLoader classLoader = undertow.getClass().getClassLoader();
            if (classLoader == null) {
                classLoader = ClassLoader.getSystemClassLoader();
            }
            Class attachmentKeyClass = classLoader.loadClass("io.undertow.util.AttachmentKey");
            Class serverRequest = classLoader.loadClass("io.undertow.servlet.handlers.ServletRequestContext");
            Object attachmentKey = serverRequest.getField("ATTACHMENT_KEY").get(null);
            Object context = Reflection.invokeMethod(http, "getAttachment", new Class[]{attachmentKeyClass}, attachmentKey);
            Object filter = Reflection.invokeMethod(context, "getCurrentServlet", new Class[]{});
            Object request = Reflection.invokeMethod(context, "getServletRequest", new Class[]{});
            Object response = Reflection.invokeMethod(context, "getServletResponse", new Class[]{});

            HookHandler.checkFilterRequest(filter, request, response);
        } catch (Exception e) {
            HookHandler.LOGGER.warn("handle undertow request failed", e);
        }
    }
}
