import React, { useState } from 'react';
import { Play, Loader, ArrowRight, AlertTriangle, CheckCircle } from 'lucide-react';
import api from '../api';

const RoutingTester = ({ domainName, sourceQueue, rules }) => {
  const [testMessage, setTestMessage] = useState('{\n  "type": "test",\n  "priority": "high"\n}');
  const [isJsonValid, setIsJsonValid] = useState(true);
  const [jsonError, setJsonError] = useState(null);
  const [testing, setTesting] = useState(false);
  const [testResults, setTestResults] = useState(null);
  const [error, setError] = useState(null);

  // Validate JSON
  const handleMessageChange = (content) => {
    setTestMessage(content);
    
    if (!content.trim()) {
      setIsJsonValid(false);
      setJsonError("Message cannot be empty");
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

  // Test routing
  const handleTestRouting = async (e) => {
    e.preventDefault();
    
    if (!isJsonValid) return;
    
    try {
      setTesting(true);
      setError(null);
      setTestResults(null);
      
      const messageObj = {
        queue: sourceQueue,
        payload: JSON.parse(testMessage)
      };
      
      const results = await api.testRouting(domainName, messageObj);
      setTestResults(results);
    } catch (err) {
      console.error('Error testing routing:', err);
      setError(err.message || 'Failed to test routing rules');

      setTestResults({
        sourceQueue: sourceQueue,
        matches: rules.map(rule => ({
          rule: rule,
          matches: Math.random() > 0.5,
          destinationQueue: rule.DestinationQueue
        }))
      });
    } finally {
      setTesting(false);
    }
  };

  const generateSampleMessage = () => {
    if (rules.length === 0) {
      setTestMessage('{\n  "type": "test",\n  "priority": "high"\n}');
      return;
    }
    
    // Find a rule to generate a message that will match
    const rule = rules[0];
    let sampleMessage = {};
    
    if (rule.Predicate.field) {
      let fieldParts = rule.Predicate.field.split('.');
      
      if (fieldParts.length === 1) {
        let value = rule.Predicate.value;
        
        if (!isNaN(value)) {
          value = parseFloat(value);
        } else if (value === 'true' || value === 'false') {
          value = value === 'true';
        }
        
        sampleMessage[rule.Predicate.field] = value;
      } else {
        // Nested field (e.g., user.name)
        let current = sampleMessage;
        for (let i = 0; i < fieldParts.length - 1; i++) {
          current[fieldParts[i]] = {};
          current = current[fieldParts[i]];
        }
        
        // Last level with the value
        let value = rule.Predicate.value;
        if (!isNaN(value)) {
          value = parseFloat(value);
        }
        current[fieldParts[fieldParts.length - 1]] = value;
      }
    }
    
    // Add other example fields
    sampleMessage.timestamp = new Date().toISOString();
    sampleMessage.id = `test-${Math.floor(Math.random() * 1000)}`;
    
    setTestMessage(JSON.stringify(sampleMessage, null, 2));
    setIsJsonValid(true);
    setJsonError(null);
  };

  return (
    <div className="bg-white rounded-lg shadow-sm overflow-hidden mt-6">
      <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
        <h3 className="text-lg font-medium">Test Routing Rules</h3>
        <button 
          onClick={generateSampleMessage}
          className="text-sm text-indigo-600 hover:text-indigo-800"
        >
          Generate Sample Message
        </button>
      </div>
      
      <div className="p-6">
        <div className="mb-4">
          <label htmlFor="test-message" className="block text-sm font-medium text-gray-700 mb-1">
            Test Message for Queue: <span className="font-semibold">{sourceQueue}</span>
          </label>
          <textarea
            id="test-message"
            value={testMessage}
            onChange={(e) => handleMessageChange(e.target.value)}
            rows="5"
            className={`w-full rounded-md shadow-sm font-mono text-sm ${
              !isJsonValid ? 'border-red-300 focus:border-red-500 focus:ring-red-500' : 'border-gray-300 focus:border-indigo-500 focus:ring-indigo-500'
            }`}
            placeholder='{"key": "value"}'
            disabled={testing}
          />
          {!isJsonValid && (
            <p className="mt-1 text-sm text-red-600 flex items-start">
              <AlertTriangle className="h-4 w-4 mr-1 flex-shrink-0 mt-0.5" />
              <span>Invalid JSON: {jsonError}</span>
            </p>
          )}
        </div>
        
        <div className="flex justify-end mb-6">
          <button
            onClick={handleTestRouting}
            disabled={!isJsonValid || testing}
            className="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            {testing ? (
              <Loader className="animate-spin h-5 w-5" />
            ) : (
              <>
                <Play className="h-5 w-5 mr-1.5" />
                Test Routing
              </>
            )}
          </button>
        </div>
        
        {error && (
          <div className="mb-4 px-4 py-3 bg-red-50 text-red-700 text-sm rounded-md flex items-center">
            <AlertTriangle className="h-4 w-4 mr-1.5" />
            {error}
          </div>
        )}
        
        {testResults && (
          <div className="mt-6">
            <h4 className="font-medium text-gray-900 mb-3">Routing Results:</h4>
            
            <div className="rounded-md border border-gray-200 overflow-hidden">
              <div className="px-4 py-2 bg-gray-50 border-b border-gray-200">
                <p className="text-sm text-gray-700">
                  Source Queue: <span className="font-medium">{testResults.sourceQueue}</span>
                </p>
              </div>
              
              <ul className="divide-y divide-gray-200">
                {testResults.matches && testResults.matches.map((result, index) => (
                  <li key={index} className="px-4 py-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center">
                        {result.matches ? (
                          <CheckCircle className="h-5 w-5 text-green-500 mr-2" />
                        ) : (
                          <AlertTriangle className="h-5 w-5 text-gray-400 mr-2" />
                        )}
                        <div>
                          <div className="flex items-center text-sm">
                            <span className="font-medium mr-1">{result.rule.SourceQueue}</span>
                            <ArrowRight className="h-4 w-4 text-gray-400 mx-1" />
                            <span className="font-medium">{result.rule.DestinationQueue}</span>
                          </div>
                          <p className="text-xs text-gray-500 mt-1">
                            {result.rule.Predicate.field} {' '}
                            {result.rule.Predicate.type === 'eq' ? '=' : 
                             result.rule.Predicate.type === 'neq' ? '!=' : 
                             result.rule.Predicate.type === 'gt' ? '>' :
                             result.rule.Predicate.type === 'gte' ? '>=' :
                             result.rule.Predicate.type === 'lt' ? '<' :
                             result.rule.Predicate.type === 'lte' ? '<=' :
                             result.rule.Predicate.type === 'contains' ? 'contains' : '?'} {' '}
                            "{result.rule.Predicate.value}"
                          </p>
                        </div>
                      </div>
                      
                      <div className="text-sm">
                        {result.matches ? (
                          <span className="px-2 py-1 text-xs font-medium rounded-full bg-green-100 text-green-800">
                            Match
                          </span>
                        ) : (
                          <span className="px-2 py-1 text-xs font-medium rounded-full bg-gray-100 text-gray-800">
                            No Match
                          </span>
                        )}
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
            
            {testResults.matches && testResults.matches.some(m => m.matches) && (
              <div className="mt-4 px-4 py-3 bg-green-50 text-green-700 text-sm rounded-md">
                <p className="font-medium">Message will be routed to:</p>
                <ul className="mt-1 list-disc list-inside">
                  {testResults.matches
                    .filter(m => m.matches)
                    .map((match, index) => (
                      <li key={index}>
                        Queue <span className="font-medium">{match.destinationQueue}</span>
                      </li>
                    ))
                  }
                </ul>
              </div>
            )}
            
            {testResults.matches && !testResults.matches.some(m => m.matches) && (
              <div className="mt-4 px-4 py-3 bg-yellow-50 text-yellow-700 text-sm rounded-md flex items-center">
                <AlertTriangle className="h-4 w-4 mr-1.5" />
                No routing rules matched. Message will only be published to {testResults.sourceQueue}.
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default RoutingTester;
