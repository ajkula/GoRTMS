import React, { useState } from 'react';
import { Send, AlertTriangle, Loader, Plus, X, Check } from 'lucide-react';
import api from '../api';

const MessagePublisher = ({ domainName, queueName, onMessagePublished }) => {
  const [messageContent, setMessageContent] = useState('{\n  "type": "notification",\n  "content": "Hello world!"\n}');
  const [isJsonValid, setIsJsonValid] = useState(true);
  const [jsonError, setJsonError] = useState(null);
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState(null);
  const [headers, setHeaders] = useState([{ key: '', value: '' }]);
  const [success, setSuccess] = useState(false);

  // Validate the JSON when editing the content
  const handleContentChange = (content) => {
    setMessageContent(content);

    if (!content.trim()) {
      setIsJsonValid(true);
      setJsonError(null);
      return;
    }

    try {
      JSON.parse(content);
      setIsJsonValid(true);
      setJsonError(null);
    } catch (err) {
      setIsJsonValid(false);
      setJsonError(err.message);
    }
  };

  const addHeader = ({ key, value }) => {
    if (key !== undefined && value !== undefined) setHeaders([...headers, { key, value }]);
  };

  const removeHeader = (index) => {
    const newHeaders = [...headers];
    newHeaders.splice(index, 1);
    setHeaders(newHeaders);
  };

  const updateHeader = (index, field, value) => {
    const newHeaders = [...headers];
    newHeaders[index][field] = value;
    setHeaders(newHeaders);
  };

  const generateSampleMessage = () => {
    const samples = [
      {
        type: "notification",
        priority: "high",
        content: "This is an urgent notification",
        timestamp: new Date().toISOString()
      },
      {
        type: "data",
        content: "Title name",
        user: {
          id: 123,
          name: "John Doe",
          email: "john@example.com"
        },
        items: [
          { id: 1, name: "Item 1", price: 19.99 },
          { id: 2, name: "Item 2", price: 29.99 }
        ]
      },
      {
        type: "event",
        name: "user.login",
        content: "user event",
        userId: "user_" + Math.floor(Math.random() * 1000),
        loginTime: new Date().toISOString(),
        ipAddress: "192.168.1." + Math.floor(Math.random() * 255)
      }
    ];

    const sample = samples[Math.floor(Math.random() * samples.length)];
    setMessageContent(JSON.stringify(sample, null, 2));
    setIsJsonValid(true);
    setJsonError(null);
  };

  const handlePublish = async (e) => {
    e.preventDefault();

    if (!messageContent.trim() || !isJsonValid) {
      return;
    }

    try {
      setPublishing(true);
      setError(null);

      let content;
      try {
        content = JSON.parse(messageContent);
      } catch (err) {
        // If it's not valid JSON, use plain text instead
        content = messageContent;
      }

      const messageHeaders = {};
      headers.forEach(header => {
        if (header.key.trim() && header.value.trim()) {
          messageHeaders[header.key] = header.value;
        }
      });

      const message = {
        content,
        headers: Object.keys(messageHeaders).length > 0 ? messageHeaders : undefined
      };

      const result = await api.publishMessage(domainName, queueName, message);

      if (onMessagePublished) {
        onMessagePublished(message);
      }
      if (result.status === "success") setSuccess(true)

    } catch (err) {
      console.error('Error publishing message:', err);
      setError(err.message || 'Failed to publish message');
    } finally {
      setPublishing(false);
      setTimeout(() => setSuccess(false), 5000);
    }
  };

  return (
    <div className="bg-white rounded-lg shadow-sm overflow-hidden">
      <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
        <h2 className="text-lg font-medium">Publish Message</h2>
        <button
          type="button"
          onClick={generateSampleMessage}
          className="text-sm text-indigo-600 hover:text-indigo-800"
        >
          Generate Sample
        </button>
      </div>

      <div className="p-6">
        <form onSubmit={handlePublish}>
          {/* message content */}
          <div className="mb-4">
            <label htmlFor="message-content" className="block text-sm font-medium text-gray-700 mb-1">
              Message Content (JSON)
            </label>
            <textarea
              id="message-content"
              rows="8"
              value={messageContent}
              onChange={(e) => handleContentChange(e.target.value)}
              placeholder='{ "key": "value" }'
              className={`w-full rounded-md shadow-sm font-mono text-sm ${!isJsonValid ? 'border-red-300 focus:border-red-500 focus:ring-red-500' : 'border-gray-300 focus:border-indigo-500 focus:ring-indigo-500'
                } focus:ring-1`}
              disabled={publishing}
            />
            {!isJsonValid && (
              <p className="mt-1 text-sm text-red-600 flex items-start">
                <AlertTriangle className="h-4 w-4 mr-1 flex-shrink-0 mt-0.5" />
                <span>Invalid JSON: {jsonError}</span>
              </p>
            )}
          </div>

          {/* headers */}
          <div className="mb-6">
            <div className="flex justify-between items-center mb-2">
              <label className="block text-sm font-medium text-gray-700">Headers (optional)</label>
              <button
                type="button"
                onClick={addHeader}
                className="text-xs text-indigo-600 hover:text-indigo-800 flex items-center"
              >
                <Plus className="h-3 w-3 mr-1" />
                Add Header
              </button>
            </div>

            {headers.map((header, index) => (
              <div key={index} className="flex space-x-2 mb-2">
                <input
                  type="text"
                  value={header.key}
                  onChange={(e) => updateHeader(index, 'key', e.target.value)}
                  placeholder="Key"
                  className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 focus:ring-1 text-sm"
                  disabled={publishing}
                />
                <input
                  type="text"
                  value={header.value}
                  onChange={(e) => updateHeader(index, 'value', e.target.value)}
                  placeholder="Value"
                  className="flex-1 rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 focus:ring-1 text-sm"
                  disabled={publishing}
                />
                {headers.length > 1 && (
                  <button
                    type="button"
                    onClick={() => removeHeader(index)}
                    className="p-1 rounded-full text-red-500 hover:bg-red-50"
                    disabled={publishing}
                  >
                    <X className="h-4 w-4" />
                  </button>
                )}
              </div>
            ))}
          </div>

          {error && (
            <div className="mb-4 px-4 py-3 bg-red-50 text-red-700 text-sm rounded-md flex items-center">
              <AlertTriangle className="h-4 w-4 mr-1.5" />
              {error}
            </div>
          )}

          {success && (
            <div className="mb-4 px-4 py-3 bg-green-50 text-green-700 text-sm rounded-md flex items-center">
              <Check className="h-4 w-4 mr-1.5" />
              Message published successfully
            </div>
          )}

          {/* publish bouton */}
          <div className="flex justify-end">
            <button
              type="submit"
              className="inline-flex justify-center items-center py-2 px-4 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
              disabled={!messageContent.trim() || !isJsonValid || publishing}
            >
              {publishing ? (
                <Loader className="h-5 w-5 animate-spin" />
              ) : (
                <>
                  <Send className="h-5 w-5 mr-1" />
                  Publish Message
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default MessagePublisher;